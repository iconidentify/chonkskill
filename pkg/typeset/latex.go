// Package typeset converts novel markdown to professional LaTeX and prepares
// files for tectonic (PDF) and pandoc (EPUB) compilation. The tool handlers
// prepare all files; the agent runs the compilation commands on the sandbox.
package typeset

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

// EscapeLatex escapes LaTeX special characters.
func EscapeLatex(s string) string {
	// Order matters: & first to avoid double-escaping.
	r := strings.NewReplacer(
		`\`, `\textbackslash{}`,
		`&`, `\&`,
		`%`, `\%`,
		`$`, `\$`,
		`#`, `\#`,
		`_`, `\_`,
		`{`, `\{`,
		`}`, `\}`,
		`~`, `\textasciitilde{}`,
		`^`, `\textasciicircum{}`,
	)
	return r.Replace(s)
}

// MarkdownToLatex converts a markdown body to LaTeX line by line.
func MarkdownToLatex(md string) string {
	lines := strings.Split(md, "\n")
	var result []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Scene breaks.
		if trimmed == "---" || trimmed == "***" || trimmed == "* * *" {
			result = append(result, `\scenebreak`)
			continue
		}

		// Skip headings (handled by ConvertChapter).
		if strings.HasPrefix(trimmed, "#") {
			continue
		}

		// Empty lines = paragraph break.
		if trimmed == "" {
			result = append(result, "")
			continue
		}

		// Convert the line content.
		converted := convertLine(trimmed)
		result = append(result, converted)
	}

	return strings.Join(result, "\n")
}

func convertLine(line string) string {
	// Escape LaTeX specials first, but preserve markdown formatting.
	// We need to handle italic markers before escaping.

	// Extract italic spans before escaping.
	line = convertItalics(line)

	// Convert quotes before escaping (quotes use special chars).
	line = convertQuotes(line)

	// Convert dashes.
	line = convertDashes(line)

	// Escape remaining LaTeX specials (but not our already-converted commands).
	line = escapeSelective(line)

	return line
}

var italicRe = regexp.MustCompile(`\*([^*]+)\*`)

func convertItalics(s string) string {
	return italicRe.ReplaceAllString(s, `\emph{$1}`)
}

func convertQuotes(s string) string {
	// Unicode curly quotes to TeX.
	s = strings.ReplaceAll(s, "\u201C", "``")  // left double
	s = strings.ReplaceAll(s, "\u201D", "''")  // right double
	s = strings.ReplaceAll(s, "\u2018", "`")   // left single
	s = strings.ReplaceAll(s, "\u2019", "'")   // right single

	// Straight double quotes: context-sensitive.
	// Opening quote after space/newline/paragraph start, closing otherwise.
	runes := []rune(s)
	var out []rune
	inQuote := false
	for i, r := range runes {
		if r == '"' {
			if !inQuote {
				// Check if this looks like an opening quote.
				if i == 0 || unicode.IsSpace(runes[i-1]) || runes[i-1] == '(' || runes[i-1] == '[' {
					out = append(out, '`', '`')
					inQuote = true
				} else {
					out = append(out, '\'', '\'')
					inQuote = false
				}
			} else {
				out = append(out, '\'', '\'')
				inQuote = false
			}
		} else {
			out = append(out, r)
		}
	}
	return string(out)
}

func convertDashes(s string) string {
	// Em dash (must come before en dash check).
	s = strings.ReplaceAll(s, "\u2014", "---")
	// En dash.
	s = strings.ReplaceAll(s, "\u2013", "--")
	// Ellipsis.
	s = strings.ReplaceAll(s, "\u2026", `\ldots{}`)
	return s
}

// escapeSelective escapes LaTeX specials but preserves already-converted
// commands (things starting with \).
func escapeSelective(s string) string {
	var out strings.Builder
	runes := []rune(s)
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		switch r {
		case '\\':
			// Already a LaTeX command -- pass through until end of command.
			out.WriteRune(r)
		case '&':
			out.WriteString(`\&`)
		case '%':
			out.WriteString(`\%`)
		case '$':
			out.WriteString(`\$`)
		case '#':
			out.WriteString(`\#`)
		case '_':
			out.WriteString(`\_`)
		default:
			out.WriteRune(r)
		}
	}
	return out.String()
}

// MakeDropCap wraps the first letter of text in a \lettrine command.
func MakeDropCap(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return text
	}

	// Find the first letter.
	runes := []rune(text)
	letterIdx := -1
	for i, r := range runes {
		if unicode.IsLetter(r) {
			letterIdx = i
			break
		}
	}
	if letterIdx < 0 {
		return text
	}

	// Find the end of the first word.
	wordEnd := letterIdx + 1
	for wordEnd < len(runes) && !unicode.IsSpace(runes[wordEnd]) {
		wordEnd++
	}

	firstLetter := string(runes[letterIdx])
	restOfWord := string(runes[letterIdx+1 : wordEnd])
	remainder := string(runes[wordEnd:])
	prefix := ""
	if letterIdx > 0 {
		prefix = string(runes[:letterIdx])
	}

	return fmt.Sprintf(`%s\lettrine[lines=2, lhang=0.1, nindent=0.2em]{%s}{%s}%s`,
		prefix, firstLetter, restOfWord, remainder)
}

// ConvertChapter converts a single chapter's markdown to LaTeX.
func ConvertChapter(num int, title, body string, hasOrnament, hasIllustration, dropCaps bool) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("\\chapter{%s}\n", EscapeLatex(title)))

	if hasOrnament {
		sb.WriteString(fmt.Sprintf("\\chapterart{ornament_ch%02d.png}\n", num))
	}

	if hasIllustration {
		sb.WriteString(fmt.Sprintf("\\chapterillustration{ch%02d.png}\n", num))
	}

	// Convert body.
	latex := MarkdownToLatex(body)

	// Apply drop cap to first paragraph if requested.
	if dropCaps {
		paragraphs := strings.SplitN(latex, "\n\n", 2)
		if len(paragraphs) > 0 && strings.TrimSpace(paragraphs[0]) != "" {
			paragraphs[0] = MakeDropCap(paragraphs[0])
			latex = strings.Join(paragraphs, "\n\n")
		}
	}

	sb.WriteString(latex)
	sb.WriteString("\n\n")
	return sb.String()
}

// BuildChaptersTeX reads all chapter files and produces the combined TeX content.
func BuildChaptersTeX(projectDir string, dropCaps bool) (string, error) {
	pattern := filepath.Join(projectDir, "chapters", "ch_*.md")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return "", err
	}
	if len(matches) == 0 {
		return "", fmt.Errorf("no chapter files found in %s/chapters/", projectDir)
	}

	// Sort by chapter number.
	re := regexp.MustCompile(`ch_(\d+)\.md$`)
	type chEntry struct {
		num  int
		path string
	}
	var chapters []chEntry
	for _, m := range matches {
		sub := re.FindStringSubmatch(filepath.Base(m))
		if sub != nil {
			n, _ := strconv.Atoi(sub[1])
			chapters = append(chapters, chEntry{n, m})
		}
	}
	sort.Slice(chapters, func(i, j int) bool { return chapters[i].num < chapters[j].num })

	artDir := filepath.Join(projectDir, "art")
	var content strings.Builder

	for _, ch := range chapters {
		data, err := os.ReadFile(ch.path)
		if err != nil {
			return "", fmt.Errorf("reading chapter %d: %w", ch.num, err)
		}

		title, body := extractChapterTitleAndBody(string(data))
		if title == "" {
			title = fmt.Sprintf("Chapter %d", ch.num)
		}

		hasOrnament := fileExists(filepath.Join(artDir, fmt.Sprintf("ornament_ch%02d.png", ch.num)))
		hasIllustration := fileExists(filepath.Join(artDir, fmt.Sprintf("ch%02d.png", ch.num)))

		content.WriteString(ConvertChapter(ch.num, title, body, hasOrnament, hasIllustration, dropCaps))
	}

	return content.String(), nil
}

// extractChapterTitleAndBody splits a chapter markdown into title and body.
func extractChapterTitleAndBody(md string) (string, string) {
	lines := strings.SplitN(md, "\n", 2)
	if len(lines) == 0 {
		return "", md
	}

	firstLine := strings.TrimSpace(lines[0])
	if strings.HasPrefix(firstLine, "#") {
		title := strings.TrimSpace(strings.TrimLeft(firstLine, "#"))
		// Remove "Chapter N:" prefix if present.
		titleRe := regexp.MustCompile(`^(?:Chapter\s+\d+\s*[:\-]\s*)(.+)$`)
		if sub := titleRe.FindStringSubmatch(title); sub != nil {
			title = sub[1]
		}
		body := ""
		if len(lines) > 1 {
			body = strings.TrimSpace(lines[1])
		}
		return title, body
	}

	return "", md
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
