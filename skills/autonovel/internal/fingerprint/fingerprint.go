// Package fingerprint provides quantitative prose analysis without LLM calls.
// It measures vocabulary well density, sentence rhythm, dialogue ratio,
// and other statistical properties of fiction prose.
package fingerprint

import (
	"fmt"
	"math"
	"regexp"
	"strings"
)

// ChapterMetrics holds per-chapter statistical analysis.
type ChapterMetrics struct {
	Chapter           int     `json:"chapter"`
	WordCount         int     `json:"word_count"`
	SentenceCount     int     `json:"sentence_count"`
	ParagraphCount    int     `json:"paragraph_count"`
	AvgSentenceLen    float64 `json:"avg_sentence_length"`
	SentenceLenStd    float64 `json:"sentence_length_std"`
	SentenceLenCV     float64 `json:"sentence_length_cv"`
	MinSentence       int     `json:"min_sentence"`
	MaxSentence       int     `json:"max_sentence"`
	FragmentsPct      float64 `json:"fragments_pct"`
	LongSentencesPct  float64 `json:"long_sentences_pct"`
	AvgParagraphLen   float64 `json:"avg_paragraph_length"`
	ParagraphLenStd   float64 `json:"paragraph_length_std"`
	WellMusicalPct    float64 `json:"well_musical_pct"`
	WellTradePct      float64 `json:"well_trade_pct"`
	WellBodyPct       float64 `json:"well_body_pct"`
	WellTotalPer1K    float64 `json:"well_total_per_1k"`
	AbstractPer1K     float64 `json:"abstract_per_1k"`
	DialogueRatio     float64 `json:"dialogue_ratio"`
	EmDashPer1K       float64 `json:"em_dash_per_1k"`
	SectionBreaks     int     `json:"section_breaks"`
	HeStartPct        float64 `json:"he_start_pct"`
	TheWayCount       int     `json:"the_way_count"`
	SimileDensity     float64 `json:"simile_density"`
}

// Outlier flags a metric that deviates significantly from the novel average.
type Outlier struct {
	Chapter   int     `json:"chapter"`
	Metric    string  `json:"metric"`
	Value     float64 `json:"value"`
	Mean      float64 `json:"mean"`
	ZScore    float64 `json:"z_score"`
	Direction string  `json:"direction"` // HIGH or LOW
}

// Report holds the full fingerprint analysis.
type Report struct {
	Chapters []ChapterMetrics `json:"chapters"`
	Averages ChapterMetrics   `json:"averages"`
	Outliers []Outlier        `json:"outliers"`
}

// Vocabulary wells.
var wellMusical = toSet([]string{
	"pitch", "tone", "interval", "harmonic", "bell", "ring", "resonance", "chord",
	"melody", "rhythm", "cadence", "note", "sound", "echo", "vibration", "hum",
	"chime", "toll", "peal", "clang", "murmur", "whisper", "silence", "harmony",
	"dissonance", "octave", "tremolo", "staccato", "legato", "crescendo", "diminuendo",
	"forte", "piano", "song", "sing", "voice", "ear", "listen", "hear", "tune",
	"strain", "refrain", "verse", "aria", "hymn", "dirge", "lullaby", "fanfare",
	"overture", "coda", "finale", "serenade", "sonata", "symphony", "concert",
})

var wellTrade = toSet([]string{
	"bronze", "metal", "forge", "contract", "ledger", "debt", "coin", "gold",
	"silver", "copper", "iron", "steel", "anvil", "hammer", "trade", "merchant",
	"market", "price", "profit", "loss", "bargain", "deal", "exchange", "barter",
	"guild", "apprentice", "master", "craft", "workshop", "tool", "mold", "cast",
	"weight", "measure", "scale", "balance", "account", "tax", "tariff", "customs",
	"cargo", "ship", "port", "dock", "warehouse", "inventory",
})

var wellBody = toSet([]string{
	"eye", "eyes", "hand", "hands", "chest", "jaw", "tongue", "breath", "pulse",
	"skin", "bone", "blood", "heart", "finger", "fingers", "arm", "arms", "leg",
	"legs", "shoulder", "shoulders", "throat", "lip", "lips", "spine", "gut",
	"knee", "knees", "palm", "palms", "wrist", "wrists", "neck", "forehead",
	"temple", "temples", "rib", "ribs", "muscle", "muscles", "nerve", "nerves",
	"vein", "veins", "tendon", "skull", "teeth", "tooth", "heel", "toe",
})

var abstractIndicators = toSet([]string{
	"sense", "feeling", "notion", "awareness", "consciousness", "impression",
	"perception", "intuition", "instinct", "premonition", "foreboding",
	"realization", "understanding", "recognition", "comprehension", "insight",
	"epiphany", "revelation", "certainty", "uncertainty", "ambiguity",
	"complexity", "profundity", "significance", "essence",
})

var sentenceSplitRe = regexp.MustCompile(`[.!?]+[\s]+`)
var dialogueRe = regexp.MustCompile(`[""` + "\u201C\u201D" + `][^""` + "\u201C\u201D" + `]*[""` + "\u201C\u201D" + `]`)
var simileRe = regexp.MustCompile(`(?i)\b(like a|as if|as though|resembled)\b`)
var sectionBreakRe = regexp.MustCompile(`(?m)^[\s]*[-*#]{3,}[\s]*$`)

// Analyze computes metrics for a single chapter text.
func Analyze(chapter int, text string) ChapterMetrics {
	words := strings.Fields(strings.ToLower(text))
	wordCount := len(words)
	sentences := splitSentences(text)
	paragraphs := splitParagraphs(text)

	m := ChapterMetrics{
		Chapter:       chapter,
		WordCount:     wordCount,
		SentenceCount: len(sentences),
		ParagraphCount: len(paragraphs),
	}

	// Sentence length stats.
	if len(sentences) > 0 {
		var lengths []float64
		fragments := 0
		longSentences := 0
		minLen := math.MaxInt32
		maxLen := 0
		for _, s := range sentences {
			wc := len(strings.Fields(s))
			lengths = append(lengths, float64(wc))
			if wc < minLen {
				minLen = wc
			}
			if wc > maxLen {
				maxLen = wc
			}
			if wc <= 4 {
				fragments++
			}
			if wc > 35 {
				longSentences++
			}
		}
		m.MinSentence = minLen
		m.MaxSentence = maxLen
		m.FragmentsPct = float64(fragments) / float64(len(sentences)) * 100
		m.LongSentencesPct = float64(longSentences) / float64(len(sentences)) * 100
		mean, std := meanStddev(lengths)
		m.AvgSentenceLen = mean
		m.SentenceLenStd = std
		if mean > 0 {
			m.SentenceLenCV = std / mean
		}
	}

	// Paragraph length stats.
	if len(paragraphs) > 0 {
		var lengths []float64
		for _, p := range paragraphs {
			lengths = append(lengths, float64(len(strings.Fields(p))))
		}
		mean, std := meanStddev(lengths)
		m.AvgParagraphLen = mean
		m.ParagraphLenStd = std
	}

	// Vocabulary well densities.
	if wordCount > 0 {
		musicalCount := countInSet(words, wellMusical)
		tradeCount := countInSet(words, wellTrade)
		bodyCount := countInSet(words, wellBody)
		abstractCount := countInSet(words, abstractIndicators)

		m.WellMusicalPct = float64(musicalCount) / float64(wordCount) * 100
		m.WellTradePct = float64(tradeCount) / float64(wordCount) * 100
		m.WellBodyPct = float64(bodyCount) / float64(wordCount) * 100
		m.WellTotalPer1K = float64(musicalCount+tradeCount+bodyCount) / float64(wordCount) * 1000
		m.AbstractPer1K = float64(abstractCount) / float64(wordCount) * 1000
	}

	// Dialogue ratio.
	dialogueMatches := dialogueRe.FindAllString(text, -1)
	dialogueWords := 0
	for _, d := range dialogueMatches {
		dialogueWords += len(strings.Fields(d))
	}
	if wordCount > 0 {
		m.DialogueRatio = float64(dialogueWords) / float64(wordCount)
	}

	// Em dash density.
	emDashes := strings.Count(text, "\u2014") + strings.Count(text, "---")
	if wordCount > 0 {
		m.EmDashPer1K = float64(emDashes) / float64(wordCount) * 1000
	}

	// Section breaks.
	m.SectionBreaks = len(sectionBreakRe.FindAllString(text, -1))

	// "He" sentence starters.
	if len(sentences) > 0 {
		heStarts := 0
		for _, s := range sentences {
			first := strings.Fields(s)
			if len(first) > 0 && (strings.EqualFold(first[0], "He") || strings.EqualFold(first[0], "She")) {
				heStarts++
			}
		}
		m.HeStartPct = float64(heStarts) / float64(len(sentences)) * 100
	}

	// "the way" count.
	m.TheWayCount = strings.Count(strings.ToLower(text), "the way")

	// Simile density.
	simileMatches := simileRe.FindAllString(text, -1)
	if wordCount > 0 {
		m.SimileDensity = float64(len(simileMatches)) / float64(wordCount) * 1000
	}

	return m
}

// AnalyzeNovel computes metrics for all chapters and finds outliers.
func AnalyzeNovel(chapters map[int]string) *Report {
	report := &Report{}

	for ch, text := range chapters {
		metrics := Analyze(ch, text)
		report.Chapters = append(report.Chapters, metrics)
	}

	// Sort by chapter number.
	for i := 0; i < len(report.Chapters); i++ {
		for j := i + 1; j < len(report.Chapters); j++ {
			if report.Chapters[j].Chapter < report.Chapters[i].Chapter {
				report.Chapters[i], report.Chapters[j] = report.Chapters[j], report.Chapters[i]
			}
		}
	}

	// Compute averages and find outliers.
	if len(report.Chapters) > 1 {
		report.Averages = computeAverages(report.Chapters)
		report.Outliers = findOutliers(report.Chapters)
	}

	return report
}

// FormatReport returns a human-readable summary of the fingerprint analysis.
func FormatReport(r *Report) string {
	var sb strings.Builder
	sb.WriteString("# Voice Fingerprint Report\n\n")
	sb.WriteString(fmt.Sprintf("Chapters analyzed: %d\n\n", len(r.Chapters)))

	sb.WriteString("## Per-Chapter Metrics\n\n")
	sb.WriteString("| Ch | Words | Sent | AvgSL | CV  | Dial% | Musical | Trade | Body | EmD/1K |\n")
	sb.WriteString("|----|-------|------|-------|-----|-------|---------|-------|------|--------|\n")
	for _, m := range r.Chapters {
		sb.WriteString(fmt.Sprintf("| %2d | %5d | %3d  | %4.1f  | %.2f | %4.1f | %5.2f   | %5.2f | %5.2f | %4.1f   |\n",
			m.Chapter, m.WordCount, m.SentenceCount, m.AvgSentenceLen,
			m.SentenceLenCV, m.DialogueRatio*100, m.WellMusicalPct,
			m.WellTradePct, m.WellBodyPct, m.EmDashPer1K))
	}

	if len(r.Outliers) > 0 {
		sb.WriteString("\n## Outliers (|z| > 1.5)\n\n")
		for _, o := range r.Outliers {
			sb.WriteString(fmt.Sprintf("- Ch %d: %s = %.2f (%s, z=%.2f, mean=%.2f)\n",
				o.Chapter, o.Metric, o.Value, o.Direction, o.ZScore, o.Mean))
		}
	}

	return sb.String()
}

func splitSentences(text string) []string {
	raw := sentenceSplitRe.Split(text, -1)
	var result []string
	for _, s := range raw {
		s = strings.TrimSpace(s)
		if len(strings.Fields(s)) >= 3 {
			result = append(result, s)
		}
	}
	return result
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

func toSet(words []string) map[string]bool {
	s := make(map[string]bool, len(words))
	for _, w := range words {
		s[w] = true
	}
	return s
}

func countInSet(words []string, set map[string]bool) int {
	count := 0
	for _, w := range words {
		if set[w] {
			count++
		}
	}
	return count
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

func computeAverages(chapters []ChapterMetrics) ChapterMetrics {
	n := float64(len(chapters))
	var avg ChapterMetrics
	for _, m := range chapters {
		avg.WordCount += m.WordCount
		avg.SentenceCount += m.SentenceCount
		avg.ParagraphCount += m.ParagraphCount
		avg.AvgSentenceLen += m.AvgSentenceLen
		avg.SentenceLenCV += m.SentenceLenCV
		avg.AvgParagraphLen += m.AvgParagraphLen
		avg.WellMusicalPct += m.WellMusicalPct
		avg.WellTradePct += m.WellTradePct
		avg.WellBodyPct += m.WellBodyPct
		avg.WellTotalPer1K += m.WellTotalPer1K
		avg.AbstractPer1K += m.AbstractPer1K
		avg.DialogueRatio += m.DialogueRatio
		avg.EmDashPer1K += m.EmDashPer1K
		avg.HeStartPct += m.HeStartPct
		avg.SimileDensity += m.SimileDensity
	}
	avg.AvgSentenceLen /= n
	avg.SentenceLenCV /= n
	avg.AvgParagraphLen /= n
	avg.WellMusicalPct /= n
	avg.WellTradePct /= n
	avg.WellBodyPct /= n
	avg.WellTotalPer1K /= n
	avg.AbstractPer1K /= n
	avg.DialogueRatio /= n
	avg.EmDashPer1K /= n
	avg.HeStartPct /= n
	avg.SimileDensity /= n
	return avg
}

type metricExtractor struct {
	name string
	fn   func(ChapterMetrics) float64
}

var metricsToCheck = []metricExtractor{
	{"avg_sentence_length", func(m ChapterMetrics) float64 { return m.AvgSentenceLen }},
	{"sentence_length_cv", func(m ChapterMetrics) float64 { return m.SentenceLenCV }},
	{"dialogue_ratio", func(m ChapterMetrics) float64 { return m.DialogueRatio }},
	{"well_musical_pct", func(m ChapterMetrics) float64 { return m.WellMusicalPct }},
	{"well_trade_pct", func(m ChapterMetrics) float64 { return m.WellTradePct }},
	{"well_body_pct", func(m ChapterMetrics) float64 { return m.WellBodyPct }},
	{"em_dash_per_1k", func(m ChapterMetrics) float64 { return m.EmDashPer1K }},
	{"abstract_per_1k", func(m ChapterMetrics) float64 { return m.AbstractPer1K }},
	{"he_start_pct", func(m ChapterMetrics) float64 { return m.HeStartPct }},
	{"simile_density", func(m ChapterMetrics) float64 { return m.SimileDensity }},
}

func findOutliers(chapters []ChapterMetrics) []Outlier {
	var outliers []Outlier
	for _, metric := range metricsToCheck {
		var vals []float64
		for _, ch := range chapters {
			vals = append(vals, metric.fn(ch))
		}
		mean, std := meanStddev(vals)
		if std == 0 {
			continue
		}
		for i, ch := range chapters {
			z := (vals[i] - mean) / std
			if math.Abs(z) > 1.5 {
				dir := "HIGH"
				if z < 0 {
					dir = "LOW"
				}
				outliers = append(outliers, Outlier{
					Chapter:   ch.Chapter,
					Metric:    metric.name,
					Value:     vals[i],
					Mean:      mean,
					ZScore:    z,
					Direction: dir,
				})
			}
		}
	}
	return outliers
}
