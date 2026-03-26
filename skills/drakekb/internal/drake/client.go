// Package drake provides a client for searching and reading the Drake Software
// Knowledge Base at kb.drakesoftware.com. The KB uses MadCap Flare with
// client-side search -- the search index is shipped as static JS files that
// the browser downloads. We fetch and parse those same files.
package drake

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/html"
)

const (
	baseURL    = "https://kb.drakesoftware.com/kb"
	numChunks  = 4
	maxResults = 25
)

// Article is a KB article from the search index.
type Article struct {
	ID         string  `json:"id"`
	Title      string  `json:"title"`
	Abstract   string  `json:"abstract"`
	URL        string  `json:"url"`
	Importance float64 `json:"importance"`
	Category   string  `json:"category"`
}

// ArticleContent is a full article fetched from the KB.
type ArticleContent struct {
	Title       string `json:"title"`
	URL         string `json:"url"`
	Description string `json:"description,omitempty"`
	Body        string `json:"body"`
	TOCPath     string `json:"toc_path,omitempty"`
}

// Client searches and reads the Drake KB.
type Client struct {
	httpClient *http.Client

	mu       sync.Mutex
	articles []Article
	loaded   bool
}

// NewClient creates a new Drake KB client.
func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *Client) fetch(url string) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d for %s", resp.StatusCode, url)
	}

	return io.ReadAll(resp.Body)
}

// loadIndex downloads and parses the SearchTopic chunk files.
func (c *Client) loadIndex() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.loaded {
		return nil
	}

	var allArticles []Article

	for i := 0; i < numChunks; i++ {
		url := fmt.Sprintf("%s/Data/SearchTopic_Chunk%d.js", baseURL, i)
		data, err := c.fetch(url)
		if err != nil {
			return fmt.Errorf("fetching chunk %d: %w", i, err)
		}

		articles, err := parseSearchTopicChunk(data)
		if err != nil {
			return fmt.Errorf("parsing chunk %d: %w", i, err)
		}
		allArticles = append(allArticles, articles...)
	}

	c.articles = allArticles
	c.loaded = true
	return nil
}

// parseSearchTopicChunk extracts articles from a MadCap Flare SearchTopic JS file.
// Format: define({"0":{y:0, u:"../Drake-Tax/12503.htm", l:-1, t:"Title", i:0.001, a:"Abstract"},...})
// The top-level keys ("0","1",...) are already quoted. The inner field names (y, u, l, t, i, a) are bare.
func parseSearchTopicChunk(data []byte) ([]Article, error) {
	content := string(data)

	// Extract the object from the define() wrapper.
	re := regexp.MustCompile(`define\(\s*(\{[\s\S]*\})\s*\)`)
	matches := re.FindStringSubmatch(content)
	if len(matches) < 2 {
		return nil, fmt.Errorf("could not find define() wrapper")
	}
	raw := matches[1]

	// The inner field names are bare single-char keys: {y:0, u:"...", ...}
	// Quote them: y: -> "y":  but only bare alpha keys, not already-quoted ones.
	raw = regexp.MustCompile(`([{,])\s*([a-zA-Z])\s*:`).ReplaceAllString(raw, `$1"$2":`)

	// Remove trailing commas before closing braces: ,} -> }
	raw = regexp.MustCompile(`,\s*}`).ReplaceAllString(raw, "}")

	// Parse the cleaned JSON.
	var entries map[string]struct {
		Y int     `json:"y"`
		U string  `json:"u"`
		L int     `json:"l"`
		T string  `json:"t"`
		I float64 `json:"i"`
		A string  `json:"a"`
	}

	if err := json.Unmarshal([]byte(raw), &entries); err != nil {
		return nil, fmt.Errorf("parsing JSON: %w", err)
	}

	var articles []Article
	for id, entry := range entries {
		// Convert relative URL to absolute.
		articleURL := entry.U
		articleURL = strings.TrimPrefix(articleURL, "../")
		fullURL := baseURL + "/" + articleURL

		// Extract category from URL path.
		category := ""
		parts := strings.Split(articleURL, "/")
		if len(parts) >= 2 {
			category = parts[0]
		}

		articles = append(articles, Article{
			ID:         id,
			Title:      entry.T,
			Abstract:   entry.A,
			URL:        fullURL,
			Importance: entry.I,
			Category:   category,
		})
	}

	return articles, nil
}

// Search finds articles matching the query string.
func (c *Client) Search(query string, category string, limit int) ([]Article, error) {
	if err := c.loadIndex(); err != nil {
		return nil, fmt.Errorf("loading search index: %w", err)
	}

	if limit <= 0 || limit > maxResults {
		limit = maxResults
	}

	query = strings.ToLower(query)
	terms := strings.Fields(query)

	type scored struct {
		article Article
		score   float64
	}

	var results []scored
	for _, a := range c.articles {
		// Filter by category if specified.
		if category != "" && !strings.EqualFold(a.Category, category) {
			continue
		}

		title := strings.ToLower(a.Title)
		abstract := strings.ToLower(a.Abstract)

		score := 0.0
		allMatch := true
		for _, term := range terms {
			titleMatch := strings.Contains(title, term)
			abstractMatch := strings.Contains(abstract, term)

			if !titleMatch && !abstractMatch {
				allMatch = false
				break
			}

			if titleMatch {
				score += 10.0 // Title matches weighted higher.
			}
			if abstractMatch {
				score += 1.0
			}
		}

		if !allMatch {
			continue
		}

		// Boost by importance from the index.
		score += a.Importance * 100

		results = append(results, scored{article: a, score: score})
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].score > results[j].score
	})

	if len(results) > limit {
		results = results[:limit]
	}

	articles := make([]Article, len(results))
	for i, r := range results {
		articles[i] = r.article
	}
	return articles, nil
}

// GetArticle fetches and extracts the content of a KB article.
func (c *Client) GetArticle(url string) (*ArticleContent, error) {
	data, err := c.fetch(url)
	if err != nil {
		return nil, fmt.Errorf("fetching article: %w", err)
	}

	return parseArticleHTML(data, url)
}

// parseArticleHTML extracts clean content from a Drake KB article page.
func parseArticleHTML(data []byte, articleURL string) (*ArticleContent, error) {
	doc, err := html.Parse(strings.NewReader(string(data)))
	if err != nil {
		return nil, fmt.Errorf("parsing HTML: %w", err)
	}

	result := &ArticleContent{URL: articleURL}

	// Extract title from <title> tag.
	var extractText func(*html.Node) string
	extractText = func(n *html.Node) string {
		if n.Type == html.TextNode {
			return n.Data
		}
		var sb strings.Builder
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			sb.WriteString(extractText(c))
		}
		return sb.String()
	}

	// Walk the DOM.
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			// Get title.
			if n.Data == "title" {
				result.Title = strings.TrimSpace(extractText(n))
			}

			// Get meta description.
			if n.Data == "meta" {
				name := ""
				content := ""
				for _, a := range n.Attr {
					if a.Key == "name" {
						name = a.Val
					}
					if a.Key == "content" {
						content = a.Val
					}
				}
				if name == "description" {
					result.Description = content
				}
			}

			// Get TOC path from html element.
			if n.Data == "html" {
				for _, a := range n.Attr {
					if a.Key == "data-mc-toc-path" {
						result.TOCPath = a.Val
					}
				}
			}

			// Extract main content.
			for _, a := range n.Attr {
				if a.Key == "id" && a.Val == "mc-main-content" {
					result.Body = extractCleanText(n)
					return
				}
			}
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)

	if result.Body == "" {
		// Fallback: extract body content.
		var findBody func(*html.Node)
		findBody = func(n *html.Node) {
			if n.Type == html.ElementNode && n.Data == "body" {
				result.Body = extractCleanText(n)
				return
			}
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				findBody(c)
			}
		}
		findBody(doc)
	}

	return result, nil
}

// extractCleanText converts an HTML node tree to readable plain text.
func extractCleanText(n *html.Node) string {
	var sb strings.Builder
	var extract func(*html.Node)

	extract = func(n *html.Node) {
		if n.Type == html.TextNode {
			text := strings.TrimSpace(n.Data)
			if text != "" {
				sb.WriteString(text)
				sb.WriteString(" ")
			}
			return
		}

		if n.Type == html.ElementNode {
			// Skip script and style tags.
			if n.Data == "script" || n.Data == "style" || n.Data == "nav" {
				return
			}

			// Add line breaks for block elements.
			isBlock := false
			switch n.Data {
			case "p", "div", "br", "h1", "h2", "h3", "h4", "h5", "h6",
				"li", "tr", "dt", "dd", "blockquote", "pre", "hr":
				isBlock = true
			}

			if isBlock {
				sb.WriteString("\n")
			}

			// Add bullet for list items.
			if n.Data == "li" {
				sb.WriteString("- ")
			}

			// Add header markers.
			switch n.Data {
			case "h1":
				sb.WriteString("# ")
			case "h2":
				sb.WriteString("## ")
			case "h3":
				sb.WriteString("### ")
			}
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			extract(c)
		}

		if n.Type == html.ElementNode {
			switch n.Data {
			case "p", "h1", "h2", "h3", "h4", "h5", "h6", "tr", "li":
				sb.WriteString("\n")
			}
		}
	}

	extract(n)

	// Clean up excessive whitespace.
	result := sb.String()
	result = regexp.MustCompile(`\n{3,}`).ReplaceAllString(result, "\n\n")
	result = regexp.MustCompile(` {2,}`).ReplaceAllString(result, " ")
	return strings.TrimSpace(result)
}

// IndexStats returns info about the loaded index.
func (c *Client) IndexStats() (total int, categories map[string]int, err error) {
	if err := c.loadIndex(); err != nil {
		return 0, nil, err
	}

	cats := make(map[string]int)
	for _, a := range c.articles {
		cats[a.Category]++
	}
	return len(c.articles), cats, nil
}
