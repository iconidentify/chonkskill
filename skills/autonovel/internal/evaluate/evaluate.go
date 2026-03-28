// Package evaluate provides the two-tier novel evaluation engine.
// Tier 1 is the mechanical slop score (no LLM). Tier 2 is Claude Opus
// as literary judge. Operates in three modes: foundation, chapter, full.
package evaluate

import (
	"fmt"
	"strings"

	"github.com/iconidentify/chonkskill/pkg/anthropic"
	"github.com/iconidentify/chonkskill/skills/autonovel/internal/slop"
)

const judgeSystem = `You are a merciless literary critic who evaluates fiction with exacting standards. You never inflate scores. A score of 5 means mediocre. A score of 8 means genuinely good. A score of 10 is reserved for published masterwork prose. You respond only in JSON.`

// FoundationResult holds the judge's evaluation of foundation materials.
type FoundationResult struct {
	WorldDepth          DimScore `json:"world_depth"`
	CharacterDepth      DimScore `json:"character_depth"`
	OutlineCompleteness DimScore `json:"outline_completeness"`
	ForeshadowBalance   DimScore `json:"foreshadowing_balance"`
	InternalConsistency DimScore `json:"internal_consistency"`
	OverallScore        float64  `json:"overall_score"`
	LoreScore           float64  `json:"lore_score"`
	WeakestDimension    string   `json:"weakest_dimension"`
	TopSuggestion       string   `json:"top_suggestion"`
	SlopScore           slop.Score `json:"slop_score"`
}

// ChapterResult holds the judge's evaluation of a single chapter.
type ChapterResult struct {
	VoiceAdherence  DimScore `json:"voice_adherence"`
	BeatCoverage    DimScore `json:"beat_coverage"`
	CharacterVoice  DimScore `json:"character_voice"`
	PlantsSeeded    DimScore `json:"plants_seeded"`
	ProseQuality    DimScore `json:"prose_quality"`
	Continuity      DimScore `json:"continuity"`
	CanonCompliance DimScore `json:"canon_compliance"`
	LoreIntegration DimScore `json:"lore_integration"`
	Engagement      DimScore `json:"engagement"`
	OverallScore    float64  `json:"overall_score"`
	WeakestDimension string  `json:"weakest_dimension"`
	Top3Revisions    []string `json:"top_3_revisions"`
	AIPatternsDetected []string `json:"ai_patterns_detected"`
	StrongestSentences []string `json:"three_strongest_sentences"`
	WeakestSentences   []string `json:"three_weakest_sentences"`
	NewCanonEntries    []string `json:"new_canon_entries"`
	SlopScore          slop.Score `json:"slop_score"`
}

// FullResult holds the judge's evaluation of the complete novel.
type FullResult struct {
	ArcCompletion        DimScore `json:"arc_completion"`
	PacingCurve          DimScore `json:"pacing_curve"`
	ThemeCoherence       DimScore `json:"theme_coherence"`
	ForeshadowResolution DimScore `json:"foreshadowing_resolution"`
	WorldConsistency     DimScore `json:"world_consistency"`
	VoiceConsistency     DimScore `json:"voice_consistency"`
	OverallEngagement    DimScore `json:"overall_engagement"`
	NovelScore           float64  `json:"novel_score"`
	WeakestChapter       int      `json:"weakest_chapter"`
	WeakestDimension     string   `json:"weakest_dimension"`
	TopSuggestion        string   `json:"top_suggestion"`
}

// DimScore is a single evaluation dimension with score, note, and fix.
type DimScore struct {
	Score         float64 `json:"score"`
	WeakestMoment string  `json:"weakest_moment,omitempty"`
	Note          string  `json:"note,omitempty"`
	Fix           string  `json:"fix,omitempty"`
}

// Foundation evaluates the world, characters, outline, and canon.
func Foundation(client *anthropic.Client, model string, voice, world, characters, outline, canon string) (*FoundationResult, error) {
	// Mechanical slop check on all layer text.
	allText := strings.Join([]string{world, characters, outline, canon}, "\n\n")
	slopResult := slop.Analyze(allText)

	prompt := fmt.Sprintf(`Evaluate these novel foundation materials. Score each dimension 1-10.

## Voice Bible
%s

## World Bible
%s

## Character Registry
%s

## Chapter Outline
%s

## Canon Database
%s

Return a JSON object with these fields:
- world_depth: {score, weakest_moment, fix}
- character_depth: {score, weakest_moment, fix}
- outline_completeness: {score, weakest_moment, fix}
- foreshadowing_balance: {score, weakest_moment, fix}
- internal_consistency: {score, weakest_moment, fix}
- overall_score: float (weighted average, world+character weighted higher)
- lore_score: float (world+canon consistency)
- weakest_dimension: string
- top_suggestion: string (single most impactful improvement)`,
		truncateForPrompt(voice, 3000),
		truncateForPrompt(world, 8000),
		truncateForPrompt(characters, 8000),
		truncateForPrompt(outline, 12000),
		truncateForPrompt(canon, 6000))

	resp, err := client.Message(anthropic.Request{
		Model:       model,
		System:      judgeSystem,
		Prompt:      prompt,
		MaxTokens:   4000,
		Temperature: 0.3,
	})
	if err != nil {
		return nil, fmt.Errorf("judge call failed: %w", err)
	}

	parsed, err := anthropic.ParseJSON(resp.Text)
	if err != nil {
		return nil, fmt.Errorf("parsing judge response: %w", err)
	}

	result := &FoundationResult{
		SlopScore: slopResult,
	}
	result.WorldDepth = extractDimScore(parsed, "world_depth")
	result.CharacterDepth = extractDimScore(parsed, "character_depth")
	result.OutlineCompleteness = extractDimScore(parsed, "outline_completeness")
	result.ForeshadowBalance = extractDimScore(parsed, "foreshadowing_balance")
	result.InternalConsistency = extractDimScore(parsed, "internal_consistency")
	result.OverallScore = extractFloat(parsed, "overall_score")
	result.LoreScore = extractFloat(parsed, "lore_score")
	result.WeakestDimension = extractString(parsed, "weakest_dimension")
	result.TopSuggestion = extractString(parsed, "top_suggestion")

	return result, nil
}

// Chapter evaluates a single chapter in context.
func Chapter(client *anthropic.Client, model string, chapterNum int, chapterText string,
	voice, world, characters, outline, canon string,
	prevChapterTail, nextChapterHead string) (*ChapterResult, error) {

	slopResult := slop.Analyze(chapterText)

	prompt := fmt.Sprintf(`Evaluate Chapter %d of this novel. Score each dimension 1-10.

## Voice Bible
%s

## World Bible (excerpt)
%s

## Character Registry (excerpt)
%s

## Chapter %d Outline Entry
%s

## Canon Database (excerpt)
%s

## Previous Chapter (last 2000 chars)
%s

## Chapter %d (THE TEXT TO EVALUATE)
%s

## Next Chapter (first 1500 chars)
%s

Return a JSON object with these fields:
- voice_adherence: {score, weakest_moment, fix}
- beat_coverage: {score, weakest_moment, fix}
- character_voice: {score, weakest_moment, fix}
- plants_seeded: {score, weakest_moment, fix}
- prose_quality: {score, weakest_moment, fix}
- continuity: {score, weakest_moment, fix}
- canon_compliance: {score, weakest_moment, fix}
- lore_integration: {score, weakest_moment, fix}
- engagement: {score, weakest_moment, fix}
- overall_score: float (weighted: prose_quality and engagement weighted highest)
- weakest_dimension: string
- top_3_revisions: [string, string, string]
- ai_patterns_detected: [string] (list any AI-sounding patterns found)
- three_strongest_sentences: [string, string, string]
- three_weakest_sentences: [string, string, string]
- new_canon_entries: [string] (new facts established in this chapter)`,
		chapterNum,
		truncateForPrompt(voice, 3000),
		truncateForPrompt(world, 4000),
		truncateForPrompt(characters, 4000),
		chapterNum, outline,
		truncateForPrompt(canon, 3000),
		prevChapterTail,
		chapterNum, chapterText,
		nextChapterHead)

	resp, err := client.Message(anthropic.Request{
		Model:       model,
		System:      judgeSystem,
		Prompt:      prompt,
		MaxTokens:   6000,
		Temperature: 0.3,
	})
	if err != nil {
		return nil, fmt.Errorf("judge call failed: %w", err)
	}

	parsed, err := anthropic.ParseJSON(resp.Text)
	if err != nil {
		return nil, fmt.Errorf("parsing judge response: %w", err)
	}

	result := &ChapterResult{
		SlopScore: slopResult,
	}
	result.VoiceAdherence = extractDimScore(parsed, "voice_adherence")
	result.BeatCoverage = extractDimScore(parsed, "beat_coverage")
	result.CharacterVoice = extractDimScore(parsed, "character_voice")
	result.PlantsSeeded = extractDimScore(parsed, "plants_seeded")
	result.ProseQuality = extractDimScore(parsed, "prose_quality")
	result.Continuity = extractDimScore(parsed, "continuity")
	result.CanonCompliance = extractDimScore(parsed, "canon_compliance")
	result.LoreIntegration = extractDimScore(parsed, "lore_integration")
	result.Engagement = extractDimScore(parsed, "engagement")
	result.OverallScore = extractFloat(parsed, "overall_score")
	result.WeakestDimension = extractString(parsed, "weakest_dimension")
	result.Top3Revisions = extractStringSlice(parsed, "top_3_revisions")
	result.AIPatternsDetected = extractStringSlice(parsed, "ai_patterns_detected")
	result.StrongestSentences = extractStringSlice(parsed, "three_strongest_sentences")
	result.WeakestSentences = extractStringSlice(parsed, "three_weakest_sentences")
	result.NewCanonEntries = extractStringSlice(parsed, "new_canon_entries")

	return result, nil
}

// Full evaluates the complete novel across all chapters.
func Full(client *anthropic.Client, model string, chapters map[int]string,
	voice, world, characters, outline, canon, arcSummary string) (*FullResult, error) {

	// Build concatenated manuscript.
	var manuscript strings.Builder
	keys := sortedKeys(chapters)
	for _, n := range keys {
		manuscript.WriteString(fmt.Sprintf("\n\n--- Chapter %d ---\n\n%s", n, chapters[n]))
	}

	prompt := fmt.Sprintf(`Evaluate this complete novel. Score each dimension 1-10.

## Voice Bible
%s

## World Bible (excerpt)
%s

## Character Registry (excerpt)
%s

## Arc Summary
%s

## Full Manuscript
%s

Return a JSON object with these fields:
- arc_completion: {score, note}
- pacing_curve: {score, note}
- theme_coherence: {score, note}
- foreshadowing_resolution: {score, note}
- world_consistency: {score, note}
- voice_consistency: {score, note}
- overall_engagement: {score, note}
- novel_score: float
- weakest_chapter: int
- weakest_dimension: string
- top_suggestion: string`,
		truncateForPrompt(voice, 3000),
		truncateForPrompt(world, 4000),
		truncateForPrompt(characters, 4000),
		truncateForPrompt(arcSummary, 6000),
		manuscript.String())

	resp, err := client.Message(anthropic.Request{
		Model:       model,
		System:      judgeSystem,
		Prompt:      prompt,
		MaxTokens:   4000,
		Temperature: 0.3,
	})
	if err != nil {
		return nil, fmt.Errorf("judge call failed: %w", err)
	}

	parsed, err := anthropic.ParseJSON(resp.Text)
	if err != nil {
		return nil, fmt.Errorf("parsing judge response: %w", err)
	}

	result := &FullResult{}
	result.ArcCompletion = extractDimScore(parsed, "arc_completion")
	result.PacingCurve = extractDimScore(parsed, "pacing_curve")
	result.ThemeCoherence = extractDimScore(parsed, "theme_coherence")
	result.ForeshadowResolution = extractDimScore(parsed, "foreshadowing_resolution")
	result.WorldConsistency = extractDimScore(parsed, "world_consistency")
	result.VoiceConsistency = extractDimScore(parsed, "voice_consistency")
	result.OverallEngagement = extractDimScore(parsed, "overall_engagement")
	result.NovelScore = extractFloat(parsed, "novel_score")
	result.WeakestChapter = int(extractFloat(parsed, "weakest_chapter"))
	result.WeakestDimension = extractString(parsed, "weakest_dimension")
	result.TopSuggestion = extractString(parsed, "top_suggestion")

	return result, nil
}

// AdversarialEdit asks the judge to find passages to cut in a chapter.
type CutResult struct {
	Cuts              []Cut  `json:"cuts"`
	TotalCuttable     int    `json:"total_cuttable_words"`
	TightestPassage   string `json:"tightest_passage"`
	LoosestPassage    string `json:"loosest_passage"`
	OverallFatPct     float64 `json:"overall_fat_percentage"`
	OneSentenceVerdict string `json:"one_sentence_verdict"`
}

type Cut struct {
	Quote   string `json:"quote"`
	Type    string `json:"type"`
	Reason  string `json:"reason"`
	Action  string `json:"action"`
	Rewrite string `json:"rewrite,omitempty"`
}

func AdversarialEdit(client *anthropic.Client, model, chapterText string, chapterNum int) (*CutResult, error) {
	prompt := fmt.Sprintf(`You are a ruthless literary editor. Read Chapter %d below and identify 10-20 passages that should be cut or rewritten. Be merciless. Good prose is lean prose.

For each passage, classify the cut type:
- FAT: unnecessary words, purple prose
- REDUNDANT: information already conveyed
- OVER-EXPLAIN: spells out what the reader can infer
- GENERIC: could appear in any novel, not specific to this one
- TELL: names an emotion instead of showing it
- STRUCTURAL: structural tic (triadic listing, balanced antithesis, etc.)

## Chapter %d
%s

Return a JSON object:
{
  "cuts": [{"quote": "exact text", "type": "FAT|REDUNDANT|...", "reason": "why cut", "action": "CUT|REWRITE", "rewrite": "replacement if REWRITE"}],
  "total_cuttable_words": N,
  "tightest_passage": "quote of the leanest paragraph",
  "loosest_passage": "quote of the most bloated paragraph",
  "overall_fat_percentage": N,
  "one_sentence_verdict": "..."
}`, chapterNum, chapterNum, chapterText)

	resp, err := client.Message(anthropic.Request{
		Model:       model,
		System:      "You are a ruthless literary editor. Your only loyalty is to the reader's experience. Respond only in JSON.",
		Prompt:      prompt,
		MaxTokens:   8000,
		Temperature: 0.3,
	})
	if err != nil {
		return nil, fmt.Errorf("judge call failed: %w", err)
	}

	parsed, err := anthropic.ParseJSON(resp.Text)
	if err != nil {
		return nil, fmt.Errorf("parsing judge response: %w", err)
	}

	result := &CutResult{
		TotalCuttable:      int(extractFloat(parsed, "total_cuttable_words")),
		TightestPassage:    extractString(parsed, "tightest_passage"),
		LoosestPassage:     extractString(parsed, "loosest_passage"),
		OverallFatPct:      extractFloat(parsed, "overall_fat_percentage"),
		OneSentenceVerdict: extractString(parsed, "one_sentence_verdict"),
	}

	if cutsRaw, ok := parsed["cuts"].([]any); ok {
		for _, c := range cutsRaw {
			if cm, ok := c.(map[string]any); ok {
				result.Cuts = append(result.Cuts, Cut{
					Quote:   extractString(cm, "quote"),
					Type:    extractString(cm, "type"),
					Reason:  extractString(cm, "reason"),
					Action:  extractString(cm, "action"),
					Rewrite: extractString(cm, "rewrite"),
				})
			}
		}
	}

	return result, nil
}

// ReaderPanel runs four persona evaluations on the arc summary.
type PanelResult struct {
	Readers       map[string]map[string]any `json:"readers"`
	Disagreements []Disagreement             `json:"disagreements"`
}

type Disagreement struct {
	Question   string   `json:"question"`
	Chapter    int      `json:"chapter"`
	FlaggedBy  []string `json:"flagged_by"`
	NotFlagged []string `json:"not_flagged"`
	Details    string   `json:"details"`
}

var readerPersonas = map[string]string{
	"editor":       "You are a senior fiction editor at a major publishing house. You care about prose texture, subtext, and sentence-level craft. You are generous but exacting.",
	"genre_reader":  "You are an avid fantasy reader who has read hundreds of novels in the genre. You care about pacing, mystery, worldbuilding payoff, and whether the book keeps you turning pages.",
	"writer":        "You are a published fantasy author reading as a craftsperson. You notice structure, technique, and the decisions behind scenes. You are collegial but honest.",
	"first_reader":  "You are a thoughtful general reader. You are not a writer or editor. You respond emotionally and honestly. You do not use craft terminology.",
}

var panelQuestions = []string{
	"momentum_loss", "earned_ending", "cut_candidate", "missing_scene",
	"thinnest_character", "best_scene", "worst_scene", "would_recommend",
	"haunts_you", "next_book",
}

func RunReaderPanel(client *anthropic.Client, model, arcSummary string) (*PanelResult, error) {
	result := &PanelResult{
		Readers: make(map[string]map[string]any),
	}

	for persona, system := range readerPersonas {
		prompt := fmt.Sprintf(`Read this arc summary of a novel, then answer each question. Return a JSON object keyed by question name.

Questions:
1. momentum_loss: Where did the novel lose momentum? (cite specific chapter numbers)
2. earned_ending: Did the ending feel earned? Why or why not?
3. cut_candidate: Which chapter(s) could be cut entirely without losing the core story?
4. missing_scene: What scene is missing that the novel needs?
5. thinnest_character: Which character feels the least developed?
6. best_scene: What was the single best scene in the novel?
7. worst_scene: What was the single worst scene?
8. would_recommend: Would you recommend this novel? To whom?
9. haunts_you: What image or moment from the novel stays with you?
10. next_book: Would you read the next book by this author?

## Arc Summary
%s`, arcSummary)

		resp, err := client.Message(anthropic.Request{
			Model:       model,
			System:      system,
			Prompt:      prompt,
			MaxTokens:   4000,
			Temperature: 0.7,
		})
		if err != nil {
			return nil, fmt.Errorf("reader %s failed: %w", persona, err)
		}

		parsed, err := anthropic.ParseJSON(resp.Text)
		if err != nil {
			// Non-fatal: record raw text.
			result.Readers[persona] = map[string]any{"raw": resp.Text}
			continue
		}
		result.Readers[persona] = parsed
	}

	// Find disagreements (chapters flagged by some but not all readers).
	disagreementQuestions := []string{"momentum_loss", "cut_candidate", "thinnest_character", "worst_scene"}
	for _, q := range disagreementQuestions {
		chapterMentions := make(map[int][]string)
		for persona, answers := range result.Readers {
			if val, ok := answers[q]; ok {
				if s, ok := val.(string); ok {
					for _, n := range extractChapterNumbers(s) {
						chapterMentions[n] = append(chapterMentions[n], persona)
					}
				}
			}
		}
		allPersonas := []string{"editor", "genre_reader", "writer", "first_reader"}
		for ch, flaggedBy := range chapterMentions {
			if len(flaggedBy) > 0 && len(flaggedBy) < len(allPersonas) {
				var notFlagged []string
				for _, p := range allPersonas {
					found := false
					for _, f := range flaggedBy {
						if f == p {
							found = true
							break
						}
					}
					if !found {
						notFlagged = append(notFlagged, p)
					}
				}
				result.Disagreements = append(result.Disagreements, Disagreement{
					Question:   q,
					Chapter:    ch,
					FlaggedBy:  flaggedBy,
					NotFlagged: notFlagged,
				})
			}
		}
	}

	return result, nil
}

// Review sends the full manuscript for deep Opus review.
type ReviewResult struct {
	Stars          float64      `json:"stars"`
	CriticSummary  string       `json:"critic_summary"`
	ProfessorItems []ReviewItem `json:"professor_items"`
	TotalItems     int          `json:"total_items"`
	MajorItems     int          `json:"major_items"`
	QualifiedItems int          `json:"qualified_items"`
	RawText        string       `json:"raw_text"`
	ShouldStop     bool         `json:"should_stop"`
	StopReason     string       `json:"stop_reason,omitempty"`
}

type ReviewItem struct {
	Text     string `json:"text"`
	Severity string `json:"severity"` // major, moderate, minor
	Type     string `json:"type"`     // compression, addition, mechanical, structural, revision
}

func Review(client *anthropic.Client, model, title, manuscript string) (*ReviewResult, error) {
	prompt := fmt.Sprintf(`Read the below novel, "%s". Review it first as a literary critic (like a newspaper book review) and then as a professor of fiction. In the later review, give specific, actionable suggestions for any defects you find. Be fair but honest. You don't *have* to find defects.

Rate the novel out of 5 stars at the start of your critic review.

## Full Manuscript
%s`, title, manuscript)

	resp, err := client.Message(anthropic.Request{
		Model:       model,
		System:      "You are an experienced literary reviewer. You review with both generosity and precision.",
		Prompt:      prompt,
		MaxTokens:   8000,
		Temperature: 0.3,
	})
	if err != nil {
		return nil, fmt.Errorf("review call failed: %w", err)
	}

	result := parseReview(resp.Text)
	return result, nil
}

func parseReview(text string) *ReviewResult {
	r := &ReviewResult{RawText: text}

	// Extract star rating.
	for _, line := range strings.Split(text, "\n") {
		stars := strings.Count(line, "\u2605") // filled star
		if stars > 0 {
			r.Stars = float64(stars)
			break
		}
		// Try "N/5" pattern.
		var n float64
		if _, err := fmt.Sscanf(line, "%f/5", &n); err == nil && n > 0 {
			r.Stars = n
			break
		}
	}

	// Split into critic and professor sections.
	lowerText := strings.ToLower(text)
	profIdx := strings.Index(lowerText, "professor")
	if profIdx > 0 {
		r.CriticSummary = strings.TrimSpace(text[:profIdx])
		profText := text[profIdx:]

		// Parse numbered items.
		items := splitNumberedItems(profText)
		for _, item := range items {
			severity := "minor"
			itemLower := strings.ToLower(item)
			if strings.Contains(itemLower, "significant") || strings.Contains(itemLower, "major") || strings.Contains(itemLower, "critical") {
				severity = "major"
			} else if strings.Contains(itemLower, "moderate") || strings.Contains(itemLower, "somewhat") {
				severity = "moderate"
			}

			itemType := "revision"
			if strings.Contains(itemLower, "compress") || strings.Contains(itemLower, "trim") || strings.Contains(itemLower, "cut") {
				itemType = "compression"
			} else if strings.Contains(itemLower, "add") || strings.Contains(itemLower, "expand") || strings.Contains(itemLower, "missing") {
				itemType = "addition"
			} else if strings.Contains(itemLower, "grammar") || strings.Contains(itemLower, "typo") || strings.Contains(itemLower, "punctuation") {
				itemType = "mechanical"
			} else if strings.Contains(itemLower, "structure") || strings.Contains(itemLower, "pacing") || strings.Contains(itemLower, "arc") {
				itemType = "structural"
			}

			qualified := strings.Contains(itemLower, "perhaps") || strings.Contains(itemLower, "might") ||
				strings.Contains(itemLower, "could") || strings.Contains(itemLower, "consider")

			ri := ReviewItem{Text: item, Severity: severity, Type: itemType}
			r.ProfessorItems = append(r.ProfessorItems, ri)
			r.TotalItems++
			if severity == "major" {
				r.MajorItems++
			}
			if qualified {
				r.QualifiedItems++
			}
		}
	} else {
		r.CriticSummary = text
	}

	// Stopping conditions.
	if r.Stars >= 4.5 && r.MajorItems == 0 {
		r.ShouldStop = true
		r.StopReason = "stars >= 4.5 and no major items"
	} else if r.Stars >= 4 && r.TotalItems > 0 && float64(r.QualifiedItems)/float64(r.TotalItems) > 0.5 {
		r.ShouldStop = true
		r.StopReason = "stars >= 4 and majority of items are qualified/hedged"
	} else if r.TotalItems <= 2 {
		r.ShouldStop = true
		r.StopReason = "2 or fewer items found"
	}

	return r
}

// CompareChapters runs a head-to-head comparison between two chapters.
type CompareResult struct {
	Winner          int    `json:"winner"`
	WinnerChapter   string `json:"winner_chapter"`
	Margin          string `json:"margin"`
	DecisiveMoment  string `json:"decisive_moment"`
	WinnerStrength  string `json:"winner_strength"`
	LoserWeakness   string `json:"loser_weakness"`
	BestSentenceA   string `json:"best_sentence_a"`
	BestSentenceB   string `json:"best_sentence_b"`
}

func CompareChapters(client *anthropic.Client, model string, chA, chB int, textA, textB string) (*CompareResult, error) {
	// Truncate to 3000 words each.
	textA = anthropic.TruncateWords(textA, 3000)
	textB = anthropic.TruncateWords(textB, 3000)

	prompt := fmt.Sprintf(`Compare these two chapters. You MUST pick a winner. No ties.

## Chapter A (Chapter %d)
%s

## Chapter B (Chapter %d)
%s

Return a JSON object:
{
  "winner": "A" or "B",
  "margin": "narrow" or "clear" or "decisive",
  "decisive_moment": "the moment that tipped the comparison",
  "winner_strength": "what the winner does best",
  "loser_weakness": "what holds the loser back",
  "best_sentence_a": "strongest sentence from A",
  "best_sentence_b": "strongest sentence from B"
}`, chA, textA, chB, textB)

	resp, err := client.Message(anthropic.Request{
		Model:       model,
		System:      "You are a literary editor comparing two chapters. You must pick a winner. Respond only in JSON.",
		Prompt:      prompt,
		MaxTokens:   4000,
		Temperature: 0.2,
	})
	if err != nil {
		return nil, fmt.Errorf("compare call failed: %w", err)
	}

	parsed, err := anthropic.ParseJSON(resp.Text)
	if err != nil {
		return nil, fmt.Errorf("parsing compare response: %w", err)
	}

	result := &CompareResult{
		Margin:         extractString(parsed, "margin"),
		DecisiveMoment: extractString(parsed, "decisive_moment"),
		WinnerStrength: extractString(parsed, "winner_strength"),
		LoserWeakness:  extractString(parsed, "loser_weakness"),
		BestSentenceA:  extractString(parsed, "best_sentence_a"),
		BestSentenceB:  extractString(parsed, "best_sentence_b"),
	}

	winner := extractString(parsed, "winner")
	if strings.EqualFold(winner, "A") {
		result.Winner = chA
		result.WinnerChapter = fmt.Sprintf("Chapter %d", chA)
	} else {
		result.Winner = chB
		result.WinnerChapter = fmt.Sprintf("Chapter %d", chB)
	}

	return result, nil
}

// Helper functions.

func extractDimScore(m map[string]any, key string) DimScore {
	val, ok := m[key]
	if !ok {
		return DimScore{}
	}
	switch v := val.(type) {
	case map[string]any:
		return DimScore{
			Score:         extractFloat(v, "score"),
			WeakestMoment: extractString(v, "weakest_moment"),
			Note:          extractString(v, "note"),
			Fix:           extractString(v, "fix"),
		}
	case float64:
		return DimScore{Score: v}
	}
	return DimScore{}
}

func extractFloat(m map[string]any, key string) float64 {
	if v, ok := m[key]; ok {
		switch n := v.(type) {
		case float64:
			return n
		case int:
			return float64(n)
		}
	}
	return 0
}

func extractString(m map[string]any, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func extractStringSlice(m map[string]any, key string) []string {
	if v, ok := m[key]; ok {
		if arr, ok := v.([]any); ok {
			var result []string
			for _, item := range arr {
				if s, ok := item.(string); ok {
					result = append(result, s)
				}
			}
			return result
		}
	}
	return nil
}

func extractChapterNumbers(text string) []int {
	// Find patterns like "chapter 5", "Ch 12", "ch. 3", or just standalone numbers in context.
	re := strings.NewReplacer("Chapter", "chapter", "Ch.", "chapter", "Ch", "chapter", "ch.", "chapter", "ch", "chapter")
	normalized := re.Replace(text)
	var nums []int
	for _, word := range strings.Fields(normalized) {
		var n int
		if _, err := fmt.Sscanf(word, "%d", &n); err == nil && n > 0 && n < 100 {
			nums = append(nums, n)
		}
	}
	return nums
}

func splitNumberedItems(text string) []string {
	lines := strings.Split(text, "\n")
	var items []string
	var current strings.Builder
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if len(trimmed) > 0 && trimmed[0] >= '1' && trimmed[0] <= '9' && strings.Contains(trimmed[:min(5, len(trimmed))], ".") {
			if current.Len() > 0 {
				items = append(items, strings.TrimSpace(current.String()))
				current.Reset()
			}
			current.WriteString(trimmed)
		} else if current.Len() > 0 {
			current.WriteString(" ")
			current.WriteString(trimmed)
		}
	}
	if current.Len() > 0 {
		items = append(items, strings.TrimSpace(current.String()))
	}
	return items
}

func truncateForPrompt(text string, maxChars int) string {
	if len(text) <= maxChars {
		return text
	}
	return text[:maxChars] + "\n[...truncated...]"
}

func sortedKeys(m map[int]string) []int {
	keys := make([]int, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	for i := 0; i < len(keys); i++ {
		for j := i + 1; j < len(keys); j++ {
			if keys[j] < keys[i] {
				keys[i], keys[j] = keys[j], keys[i]
			}
		}
	}
	return keys
}
