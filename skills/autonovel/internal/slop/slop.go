// Package slop implements mechanical AI-slop detection without LLM calls.
// It replicates the tier-based penalty system from autonovel's evaluate.py:
// banned words, suspicious clusters, filler phrases, fiction AI tells,
// structural tics, and telling patterns.
package slop

import (
	"math"
	"regexp"
	"strings"
	"unicode"
)

// Score holds the results of a mechanical slop analysis.
type Score struct {
	Tier1Hits            int      `json:"tier1_hits"`
	Tier1Words           []string `json:"tier1_words,omitempty"`
	Tier2Hits            int      `json:"tier2_hits"`
	Tier2Clusters        int      `json:"tier2_clusters"`
	Tier3Hits            int      `json:"tier3_hits"`
	FictionAITells       int      `json:"fiction_ai_tells"`
	StructuralAITics     int      `json:"structural_ai_tics"`
	TellingViolations    int      `json:"telling_violations"`
	EmDashDensity        float64  `json:"em_dash_density"`
	SentenceLengthCV     float64  `json:"sentence_length_cv"`
	TransitionOpenerPct  float64  `json:"transition_opener_pct"`
	SlopPenalty          float64  `json:"slop_penalty"`
}

// Tier1 banned words: 1.5 pts each, max 4.0 total.
var tier1Banned = []string{
	"delve", "utilize", "leverage", "facilitate", "aforementioned",
	"comprehensive", "paramount", "synergy", "paradigm", "holistic",
	"nuanced", "multifaceted", "intricate", "pivotal", "commences",
	"culminates", "endeavor", "myriad", "plethora",
}

// Tier2 suspicious words: flagged when 3+ appear in a single paragraph.
// 1.0 pt per cluster, max 2.0 total.
var tier2Suspicious = []string{
	"robust", "comprehensive", "seamless", "innovative", "cutting-edge",
	"state-of-the-art", "groundbreaking", "transformative", "dynamic",
	"streamline", "optimize", "empower", "enhance", "foster",
	"cultivate", "navigate", "landscape", "ecosystem", "framework",
	"stakeholder", "deliverable", "actionable",
}

// Tier3 filler phrases: 0.3 pts each, max 2.0 total.
var tier3Filler = []*regexp.Regexp{
	regexp.MustCompile(`(?i)it'?s worth noting that`),
	regexp.MustCompile(`(?i)let'?s dive into`),
	regexp.MustCompile(`(?i)in today'?s world`),
	regexp.MustCompile(`(?i)at the end of the day`),
	regexp.MustCompile(`(?i)it goes without saying`),
	regexp.MustCompile(`(?i)needless to say`),
	regexp.MustCompile(`(?i)in conclusion`),
	regexp.MustCompile(`(?i)to summarize`),
	regexp.MustCompile(`(?i)in summary`),
	regexp.MustCompile(`(?i)as we can see`),
	regexp.MustCompile(`(?i)it is important to note`),
	regexp.MustCompile(`(?i)this is a testament to`),
	regexp.MustCompile(`(?i)at its core`),
	regexp.MustCompile(`(?i)when it comes to`),
	regexp.MustCompile(`(?i)in the realm of`),
	regexp.MustCompile(`(?i)a testament to`),
	regexp.MustCompile(`(?i)serves as a reminder`),
	regexp.MustCompile(`(?i)the fact that`),
	regexp.MustCompile(`(?i)it should be noted that`),
}

// Transition openers: flagged if >30% of paragraphs start with one.
var transitionOpeners = []string{
	"however", "furthermore", "moreover", "additionally",
	"consequently", "nevertheless", "meanwhile", "subsequently",
}

// Fiction-specific AI tells: 0.3 pts each, max 2.0 total.
var fictionAITells = []*regexp.Regexp{
	regexp.MustCompile(`(?i)a sense of \w+`),
	regexp.MustCompile(`(?i)eyes widened`),
	regexp.MustCompile(`(?i)heart pounded in`),
	regexp.MustCompile(`(?i)couldn'?t help but`),
	regexp.MustCompile(`(?i)let out a breath`),
	regexp.MustCompile(`(?i)a flicker of`),
	regexp.MustCompile(`(?i)a wave of \w+ washed`),
	regexp.MustCompile(`(?i)his jaw clenched`),
	regexp.MustCompile(`(?i)her jaw clenched`),
	regexp.MustCompile(`(?i)the weight of .{5,30} settled`),
	regexp.MustCompile(`(?i)something shifted`),
	regexp.MustCompile(`(?i)a mixture of .+ and`),
	regexp.MustCompile(`(?i)the air .{0,20}(thick|heavy|charged)`),
	regexp.MustCompile(`(?i)silence stretched`),
	regexp.MustCompile(`(?i)darkness closed in`),
}

// Structural AI tics: 0.5 pts each, max 2.0 total.
var structuralAITics = []*regexp.Regexp{
	regexp.MustCompile(`(?i)not just .{3,40}, but`),
	regexp.MustCompile(`(?i)there'?s a difference between .+ and`),
	regexp.MustCompile(`(?i)it wasn'?t .{3,40}[.—] it was`),
	regexp.MustCompile(`(?i)the question wasn'?t .{3,40}[.—] it was`),
	regexp.MustCompile(`(?i)and yet,`),
	regexp.MustCompile(`(?i)but then again,`),
}

// Telling patterns: 0.2 pts each, max 1.5 total.
var tellingPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(felt|feeling) (a |the )?(sudden |deep |growing )?(sense|wave|surge|rush|pang) of (anger|sadness|fear|joy|relief|dread|guilt|shame|hope|love|rage|despair|terror)`),
	regexp.MustCompile(`(?i)\w+ly (said|replied|answered|whispered|shouted|muttered|growled|hissed)`),
}

// Analyze runs all mechanical slop checks on the given text and returns a Score.
func Analyze(text string) Score {
	lower := strings.ToLower(text)
	paragraphs := splitParagraphs(text)
	sentences := splitSentences(text)

	var s Score

	// Tier 1: banned words.
	for _, word := range tier1Banned {
		count := countWord(lower, word)
		if count > 0 {
			s.Tier1Hits += count
			s.Tier1Words = append(s.Tier1Words, word)
		}
	}
	t1 := math.Min(float64(s.Tier1Hits)*1.5, 4.0)

	// Tier 2: suspicious word clusters per paragraph.
	for _, para := range paragraphs {
		paraLower := strings.ToLower(para)
		hits := 0
		for _, word := range tier2Suspicious {
			if strings.Contains(paraLower, word) {
				hits++
			}
		}
		s.Tier2Hits += hits
		if hits >= 3 {
			s.Tier2Clusters++
		}
	}
	t2 := math.Min(float64(s.Tier2Clusters)*1.0, 2.0)

	// Tier 3: filler phrases.
	for _, re := range tier3Filler {
		s.Tier3Hits += len(re.FindAllString(text, -1))
	}
	t3 := math.Min(float64(s.Tier3Hits)*0.3, 2.0)

	// Fiction AI tells.
	for _, re := range fictionAITells {
		s.FictionAITells += len(re.FindAllString(text, -1))
	}
	t4 := math.Min(float64(s.FictionAITells)*0.3, 2.0)

	// Structural AI tics.
	for _, re := range structuralAITics {
		s.StructuralAITics += len(re.FindAllString(text, -1))
	}
	t5 := math.Min(float64(s.StructuralAITics)*0.5, 2.0)

	// Telling patterns.
	for _, re := range tellingPatterns {
		s.TellingViolations += len(re.FindAllString(text, -1))
	}
	t6 := math.Min(float64(s.TellingViolations)*0.2, 1.5)

	// Em dash density (per 1000 words).
	wordCount := len(strings.Fields(text))
	emDashCount := strings.Count(text, "\u2014") + strings.Count(text, "---")
	if wordCount > 0 {
		s.EmDashDensity = float64(emDashCount) / float64(wordCount) * 1000
	}

	// Sentence length coefficient of variation.
	if len(sentences) > 1 {
		var lengths []float64
		for _, sent := range sentences {
			lengths = append(lengths, float64(len(strings.Fields(sent))))
		}
		mean, stddev := meanStddev(lengths)
		if mean > 0 {
			s.SentenceLengthCV = stddev / mean
		}
	}

	// Transition opener ratio.
	if len(paragraphs) > 0 {
		transCount := 0
		for _, para := range paragraphs {
			firstWord := strings.ToLower(firstWordOf(para))
			for _, opener := range transitionOpeners {
				if firstWord == opener {
					transCount++
					break
				}
			}
		}
		s.TransitionOpenerPct = float64(transCount) / float64(len(paragraphs))
	}

	// Composite penalty, capped at 10.0.
	s.SlopPenalty = math.Min(t1+t2+t3+t4+t5+t6, 10.0)

	return s
}

func splitParagraphs(text string) []string {
	raw := strings.Split(text, "\n\n")
	var result []string
	for _, p := range raw {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

var sentenceSplitter = regexp.MustCompile(`[.!?]+[\s]+`)

func splitSentences(text string) []string {
	raw := sentenceSplitter.Split(text, -1)
	var result []string
	for _, s := range raw {
		s = strings.TrimSpace(s)
		if len(strings.Fields(s)) >= 3 {
			result = append(result, s)
		}
	}
	return result
}

func countWord(lower, word string) int {
	count := 0
	wl := strings.ToLower(word)
	idx := 0
	for {
		pos := strings.Index(lower[idx:], wl)
		if pos < 0 {
			break
		}
		absPos := idx + pos
		// Check word boundaries.
		before := absPos == 0 || !unicode.IsLetter(rune(lower[absPos-1]))
		afterIdx := absPos + len(wl)
		after := afterIdx >= len(lower) || !unicode.IsLetter(rune(lower[afterIdx]))
		if before && after {
			count++
		}
		idx = absPos + len(wl)
	}
	return count
}

func firstWordOf(text string) string {
	fields := strings.Fields(text)
	if len(fields) == 0 {
		return ""
	}
	return strings.TrimFunc(fields[0], func(r rune) bool {
		return !unicode.IsLetter(r)
	})
}

func meanStddev(vals []float64) (float64, float64) {
	if len(vals) == 0 {
		return 0, 0
	}
	sum := 0.0
	for _, v := range vals {
		sum += v
	}
	mean := sum / float64(len(vals))

	variance := 0.0
	for _, v := range vals {
		d := v - mean
		variance += d * d
	}
	variance /= float64(len(vals))
	return mean, math.Sqrt(variance)
}
