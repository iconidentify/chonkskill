// Package readability provides reading level analysis for children's literature.
// Implements Flesch-Kincaid Grade Level, Coleman-Liau Index, Automated Readability
// Index, syllable counting, and grade-level vocabulary checking.
package readability

import (
	"fmt"
	"math"
	"regexp"
	"strings"
	"unicode"
)

// GradeSpec defines the target parameters for a specific grade level.
type GradeSpec struct {
	Grade             int
	Label             string  // "Grade 3", "Grade 4", etc.
	FKMin             float64 // Flesch-Kincaid Grade Level minimum
	FKMax             float64 // Flesch-Kincaid Grade Level maximum
	AvgSentenceLen    float64 // Target average sentence length (words)
	MaxSentenceLen    int     // Longest acceptable sentence
	MaxSyllablesPerWord float64 // Average syllables per word target
	ChapterWordTarget int     // Target words per chapter
	BookWordMin       int     // Minimum total book length
	BookWordMax       int     // Maximum total book length
	ChapterCount      int     // Typical chapter count
	DialoguePctMin    float64 // Minimum dialogue percentage (kids books need more)
	DialoguePctMax    float64 // Maximum
	VocabComplexity   float64 // Max percentage of words above grade level
}

// GradeSpecs for grades 3-6.
var GradeSpecs = map[int]GradeSpec{
	3: {
		Grade: 3, Label: "Grade 3",
		FKMin: 2.0, FKMax: 3.9,
		AvgSentenceLen: 9, MaxSentenceLen: 18,
		MaxSyllablesPerWord: 1.3,
		ChapterWordTarget: 800, BookWordMin: 5000, BookWordMax: 12000,
		ChapterCount: 10, DialoguePctMin: 30, DialoguePctMax: 55,
		VocabComplexity: 5,
	},
	4: {
		Grade: 4, Label: "Grade 4",
		FKMin: 3.0, FKMax: 4.9,
		AvgSentenceLen: 11, MaxSentenceLen: 22,
		MaxSyllablesPerWord: 1.4,
		ChapterWordTarget: 1200, BookWordMin: 8000, BookWordMax: 18000,
		ChapterCount: 12, DialoguePctMin: 25, DialoguePctMax: 50,
		VocabComplexity: 8,
	},
	5: {
		Grade: 5, Label: "Grade 5",
		FKMin: 4.0, FKMax: 5.9,
		AvgSentenceLen: 13, MaxSentenceLen: 28,
		MaxSyllablesPerWord: 1.5,
		ChapterWordTarget: 1800, BookWordMin: 15000, BookWordMax: 30000,
		ChapterCount: 15, DialoguePctMin: 20, DialoguePctMax: 45,
		VocabComplexity: 12,
	},
	6: {
		Grade: 6, Label: "Grade 6",
		FKMin: 5.0, FKMax: 6.9,
		AvgSentenceLen: 15, MaxSentenceLen: 32,
		MaxSyllablesPerWord: 1.6,
		ChapterWordTarget: 2200, BookWordMin: 20000, BookWordMax: 45000,
		ChapterCount: 18, DialoguePctMin: 18, DialoguePctMax: 42,
		VocabComplexity: 15,
	},
}

// Analysis holds the complete readability analysis of a text.
type Analysis struct {
	// Core metrics.
	WordCount      int     `json:"word_count"`
	SentenceCount  int     `json:"sentence_count"`
	SyllableCount  int     `json:"syllable_count"`
	ParagraphCount int     `json:"paragraph_count"`
	AvgSentenceLen float64 `json:"avg_sentence_length"`
	AvgSyllables   float64 `json:"avg_syllables_per_word"`
	MaxSentenceLen int     `json:"max_sentence_length"`
	DialoguePct    float64 `json:"dialogue_pct"`

	// Grade level indices.
	FleschKincaid float64 `json:"flesch_kincaid_grade"`
	ColemanLiau   float64 `json:"coleman_liau_index"`
	ARI           float64 `json:"automated_readability_index"`
	ConsensusGrade float64 `json:"consensus_grade"` // Average of the three

	// Vocabulary analysis.
	ComplexWords      int      `json:"complex_words"`       // 3+ syllables
	ComplexPct        float64  `json:"complex_word_pct"`
	AboveGradeWords   []string `json:"above_grade_words,omitempty"` // Sample of words above target grade
	AboveGradePct     float64  `json:"above_grade_word_pct"`

	// Long sentences (above max for grade).
	LongSentences     int     `json:"long_sentences"`
	LongSentencePct   float64 `json:"long_sentence_pct"`

	// Grade fit assessment.
	TargetGrade       int     `json:"target_grade"`
	GradeFit          string  `json:"grade_fit"` // "on-target", "too-easy", "too-hard"
	Issues            []Issue `json:"issues,omitempty"`
}

// Issue is a specific readability problem found in the text.
type Issue struct {
	Type     string `json:"type"`     // "sentence_too_long", "vocabulary_too_hard", etc.
	Severity string `json:"severity"` // "warning", "error"
	Detail   string `json:"detail"`
	Location string `json:"location,omitempty"` // First few words of the problematic text
}

// Analyze performs full readability analysis against a target grade level.
func Analyze(text string, targetGrade int) *Analysis {
	spec, ok := GradeSpecs[targetGrade]
	if !ok {
		spec = GradeSpecs[4] // Default to grade 4
	}

	words := tokenizeWords(text)
	sentences := splitSentences(text)
	paragraphs := splitParagraphs(text)

	a := &Analysis{
		WordCount:      len(words),
		SentenceCount:  len(sentences),
		ParagraphCount: len(paragraphs),
		TargetGrade:    targetGrade,
	}

	if a.WordCount == 0 || a.SentenceCount == 0 {
		a.GradeFit = "insufficient-text"
		return a
	}

	// Syllable analysis.
	totalSyllables := 0
	complexWords := 0
	var aboveGradeSample []string
	for _, w := range words {
		syl := CountSyllables(w)
		totalSyllables += syl
		if syl >= 3 {
			complexWords++
		}
		if isAboveGradeLevel(w, targetGrade) && len(aboveGradeSample) < 20 {
			aboveGradeSample = append(aboveGradeSample, w)
		}
	}
	a.SyllableCount = totalSyllables
	a.ComplexWords = complexWords
	a.AvgSyllables = float64(totalSyllables) / float64(a.WordCount)
	a.ComplexPct = float64(complexWords) / float64(a.WordCount) * 100
	a.AboveGradeWords = aboveGradeSample
	a.AboveGradePct = float64(len(aboveGradeSample)) / float64(min(a.WordCount, 200)) * 100

	// Sentence analysis.
	a.AvgSentenceLen = float64(a.WordCount) / float64(a.SentenceCount)
	longSentences := 0
	maxLen := 0
	for _, sent := range sentences {
		wc := len(strings.Fields(sent))
		if wc > maxLen {
			maxLen = wc
		}
		if wc > spec.MaxSentenceLen {
			longSentences++
		}
	}
	a.MaxSentenceLen = maxLen
	a.LongSentences = longSentences
	a.LongSentencePct = float64(longSentences) / float64(a.SentenceCount) * 100

	// Dialogue percentage.
	dialogueWords := countDialogueWords(text)
	a.DialoguePct = float64(dialogueWords) / float64(a.WordCount) * 100

	// Flesch-Kincaid Grade Level.
	// FK = 0.39 * (words/sentences) + 11.8 * (syllables/words) - 15.59
	a.FleschKincaid = 0.39*a.AvgSentenceLen + 11.8*a.AvgSyllables - 15.59
	a.FleschKincaid = math.Round(a.FleschKincaid*10) / 10

	// Coleman-Liau Index.
	// CLI = 0.0588 * L - 0.296 * S - 15.8
	// L = avg letters per 100 words, S = avg sentences per 100 words
	charCount := countLetters(text)
	L := float64(charCount) / float64(a.WordCount) * 100
	S := float64(a.SentenceCount) / float64(a.WordCount) * 100
	a.ColemanLiau = 0.0588*L - 0.296*S - 15.8
	a.ColemanLiau = math.Round(a.ColemanLiau*10) / 10

	// Automated Readability Index.
	// ARI = 4.71 * (characters/words) + 0.5 * (words/sentences) - 21.43
	a.ARI = 4.71*(float64(charCount)/float64(a.WordCount)) + 0.5*a.AvgSentenceLen - 21.43
	a.ARI = math.Round(a.ARI*10) / 10

	// Consensus grade.
	a.ConsensusGrade = math.Round((a.FleschKincaid+a.ColemanLiau+a.ARI)/3*10) / 10

	// Grade fit assessment.
	a.GradeFit = assessGradeFit(a, spec)
	a.Issues = findIssues(a, spec, sentences)

	return a
}

// FormatReport returns a human-readable readability report.
func FormatReport(a *Analysis) string {
	spec := GradeSpecs[a.TargetGrade]
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# Readability Report (Target: %s)\n\n", spec.Label))
	sb.WriteString(fmt.Sprintf("Grade fit: **%s**\n\n", a.GradeFit))

	sb.WriteString("## Metrics\n")
	sb.WriteString(fmt.Sprintf("| Metric | Value | Target |\n"))
	sb.WriteString(fmt.Sprintf("|--------|-------|--------|\n"))
	sb.WriteString(fmt.Sprintf("| Words | %d | %d-%d (book) |\n", a.WordCount, spec.BookWordMin, spec.BookWordMax))
	sb.WriteString(fmt.Sprintf("| FK Grade | %.1f | %.1f-%.1f |\n", a.FleschKincaid, spec.FKMin, spec.FKMax))
	sb.WriteString(fmt.Sprintf("| Coleman-Liau | %.1f | - |\n", a.ColemanLiau))
	sb.WriteString(fmt.Sprintf("| ARI | %.1f | - |\n", a.ARI))
	sb.WriteString(fmt.Sprintf("| Consensus | %.1f | %d.0-%d.9 |\n", a.ConsensusGrade, spec.Grade-1, spec.Grade))
	sb.WriteString(fmt.Sprintf("| Avg Sentence | %.1f words | ~%.0f |\n", a.AvgSentenceLen, spec.AvgSentenceLen))
	sb.WriteString(fmt.Sprintf("| Max Sentence | %d words | <%d |\n", a.MaxSentenceLen, spec.MaxSentenceLen))
	sb.WriteString(fmt.Sprintf("| Avg Syllables/Word | %.2f | <%.1f |\n", a.AvgSyllables, spec.MaxSyllablesPerWord))
	sb.WriteString(fmt.Sprintf("| Complex Words | %.1f%% | <%.0f%% |\n", a.ComplexPct, spec.VocabComplexity))
	sb.WriteString(fmt.Sprintf("| Dialogue | %.1f%% | %.0f-%.0f%% |\n", a.DialoguePct, spec.DialoguePctMin, spec.DialoguePctMax))

	if len(a.Issues) > 0 {
		sb.WriteString("\n## Issues\n")
		for _, issue := range a.Issues {
			sb.WriteString(fmt.Sprintf("- [%s] %s: %s\n", issue.Severity, issue.Type, issue.Detail))
		}
	}

	if len(a.AboveGradeWords) > 0 {
		sb.WriteString("\n## Above-Grade Vocabulary\n")
		sb.WriteString(strings.Join(a.AboveGradeWords, ", ") + "\n")
	}

	return sb.String()
}

// GradeConstraints returns a prompt fragment describing the writing constraints
// for a specific grade level.
func GradeConstraints(grade int) string {
	spec, ok := GradeSpecs[grade]
	if !ok {
		spec = GradeSpecs[4]
	}

	return fmt.Sprintf(`READING LEVEL: %s (Flesch-Kincaid %.1f-%.1f)

SENTENCE RULES:
- Average sentence length: ~%d words
- Maximum sentence length: %d words
- Mix short sentences (4-6 words) with medium ones. Short sentences build tension.
- Never use semicolons. Rare use of colons.

VOCABULARY RULES:
- Use mostly 1-2 syllable words
- Maximum average syllables per word: %.1f
- No more than %.0f%% of words should be 3+ syllables
- When you must use a harder word, the context must make its meaning clear
- No vocabulary that requires a dictionary lookup for this age group

DIALOGUE RULES:
- %.0f-%.0f%% of the text should be dialogue
- Kids at this level engage most with character conversations
- Dialogue tags: stick to "said" and "asked" -- avoid "exclaimed", "retorted", etc.
- Characters should sound like real people, not textbooks

CHAPTER RULES:
- Target ~%d words per chapter
- Each chapter should have a clear mini-arc (problem -> attempt -> outcome)
- End each chapter with a reason to keep reading
- Chapter titles should be fun and hint at what happens

BOOK STRUCTURE:
- Target %d chapters, %d-%d total words
- Clear three-act structure but with faster pacing than adult fiction
- Every chapter must advance the plot -- no filler
- Subplots should be simple and clearly connected to the main story`,
		spec.Label, spec.FKMin, spec.FKMax,
		int(spec.AvgSentenceLen), spec.MaxSentenceLen,
		spec.MaxSyllablesPerWord, spec.VocabComplexity,
		spec.DialoguePctMin, spec.DialoguePctMax,
		spec.ChapterWordTarget,
		spec.ChapterCount, spec.BookWordMin, spec.BookWordMax)
}

// CountSyllables counts syllables in a word using a rule-based approach.
func CountSyllables(word string) int {
	word = strings.ToLower(strings.TrimFunc(word, func(r rune) bool {
		return !unicode.IsLetter(r)
	}))
	if len(word) <= 2 {
		return 1
	}

	// Count vowel groups.
	vowels := "aeiouy"
	count := 0
	prevVowel := false
	for _, ch := range word {
		isVowel := strings.ContainsRune(vowels, ch)
		if isVowel && !prevVowel {
			count++
		}
		prevVowel = isVowel
	}

	// Adjustments.
	// Silent e at end.
	if strings.HasSuffix(word, "e") && !strings.HasSuffix(word, "le") {
		count--
	}
	// -ed that doesn't add a syllable (walked, jumped).
	if strings.HasSuffix(word, "ed") && len(word) > 3 {
		beforeEd := word[len(word)-3]
		if beforeEd != 't' && beforeEd != 'd' {
			count--
		}
	}
	// -es that doesn't add a syllable.
	if strings.HasSuffix(word, "es") && !strings.HasSuffix(word, "ses") && !strings.HasSuffix(word, "zes") && !strings.HasSuffix(word, "ces") {
		// Don't subtract, it's usually silent.
	}

	if count < 1 {
		count = 1
	}
	return count
}

func assessGradeFit(a *Analysis, spec GradeSpec) string {
	if a.ConsensusGrade < spec.FKMin-0.5 {
		return "too-easy"
	}
	if a.ConsensusGrade > spec.FKMax+0.5 {
		return "too-hard"
	}
	return "on-target"
}

func findIssues(a *Analysis, spec GradeSpec, sentences []string) []Issue {
	var issues []Issue

	if a.FleschKincaid > spec.FKMax+1 {
		issues = append(issues, Issue{
			Type:     "reading_level_too_high",
			Severity: "error",
			Detail:   fmt.Sprintf("FK grade %.1f exceeds %s max of %.1f", a.FleschKincaid, spec.Label, spec.FKMax),
		})
	} else if a.FleschKincaid > spec.FKMax {
		issues = append(issues, Issue{
			Type:     "reading_level_slightly_high",
			Severity: "warning",
			Detail:   fmt.Sprintf("FK grade %.1f is above %s target range (%.1f-%.1f)", a.FleschKincaid, spec.Label, spec.FKMin, spec.FKMax),
		})
	}

	if a.FleschKincaid < spec.FKMin-1 {
		issues = append(issues, Issue{
			Type:     "reading_level_too_low",
			Severity: "warning",
			Detail:   fmt.Sprintf("FK grade %.1f is below %s -- may feel too simple", a.FleschKincaid, spec.Label),
		})
	}

	if a.AvgSentenceLen > spec.AvgSentenceLen*1.3 {
		issues = append(issues, Issue{
			Type:     "sentences_too_long",
			Severity: "error",
			Detail:   fmt.Sprintf("Average sentence %.1f words (target ~%.0f)", a.AvgSentenceLen, spec.AvgSentenceLen),
		})
	}

	if a.LongSentencePct > 15 {
		issues = append(issues, Issue{
			Type:     "too_many_long_sentences",
			Severity: "warning",
			Detail:   fmt.Sprintf("%.0f%% of sentences exceed %d words", a.LongSentencePct, spec.MaxSentenceLen),
		})
	}

	if a.ComplexPct > spec.VocabComplexity*1.5 {
		issues = append(issues, Issue{
			Type:     "vocabulary_too_complex",
			Severity: "error",
			Detail:   fmt.Sprintf("%.1f%% complex words (target <%.0f%%)", a.ComplexPct, spec.VocabComplexity),
		})
	}

	if a.DialoguePct < spec.DialoguePctMin {
		issues = append(issues, Issue{
			Type:     "too_little_dialogue",
			Severity: "warning",
			Detail:   fmt.Sprintf("%.0f%% dialogue (target %.0f-%.0f%%)", a.DialoguePct, spec.DialoguePctMin, spec.DialoguePctMax),
		})
	}

	if a.AvgSyllables > spec.MaxSyllablesPerWord {
		issues = append(issues, Issue{
			Type:     "words_too_complex",
			Severity: "warning",
			Detail:   fmt.Sprintf("%.2f avg syllables/word (target <%.1f)", a.AvgSyllables, spec.MaxSyllablesPerWord),
		})
	}

	// Flag specific long sentences.
	for _, sent := range sentences {
		wc := len(strings.Fields(sent))
		if wc > spec.MaxSentenceLen+5 {
			preview := sent
			if len(preview) > 60 {
				preview = preview[:60] + "..."
			}
			issues = append(issues, Issue{
				Type:     "sentence_too_long",
				Severity: "error",
				Detail:   fmt.Sprintf("%d words (max %d)", wc, spec.MaxSentenceLen),
				Location: preview,
			})
			if len(issues) > 20 {
				break
			}
		}
	}

	return issues
}

// isAboveGradeLevel checks if a word is likely above the target grade.
// Uses syllable count as primary heuristic plus a curated hard-word list.
func isAboveGradeLevel(word string, grade int) bool {
	word = strings.ToLower(word)
	syl := CountSyllables(word)

	switch {
	case grade <= 3:
		return syl >= 3 && len(word) >= 7
	case grade == 4:
		return syl >= 3 && len(word) >= 8
	case grade == 5:
		return syl >= 4 && len(word) >= 9
	default:
		return syl >= 4 && len(word) >= 10
	}
}

var sentenceEnd = regexp.MustCompile(`[.!?]+[\s]+|[.!?]+$`)

func splitSentences(text string) []string {
	raw := sentenceEnd.Split(text, -1)
	var result []string
	for _, s := range raw {
		s = strings.TrimSpace(s)
		if len(strings.Fields(s)) >= 2 {
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

func tokenizeWords(text string) []string {
	fields := strings.Fields(text)
	var words []string
	for _, f := range fields {
		w := strings.TrimFunc(f, func(r rune) bool {
			return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '\''
		})
		if w != "" {
			words = append(words, w)
		}
	}
	return words
}

var dialoguePattern = regexp.MustCompile(`[""` + "\u201C\u201D" + `]([^""` + "\u201C\u201D" + `]*)[""` + "\u201C\u201D" + `]`)

func countDialogueWords(text string) int {
	matches := dialoguePattern.FindAllStringSubmatch(text, -1)
	count := 0
	for _, m := range matches {
		if len(m) > 1 {
			count += len(strings.Fields(m[1]))
		}
	}
	return count
}

func countLetters(text string) int {
	count := 0
	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			count++
		}
	}
	return count
}
