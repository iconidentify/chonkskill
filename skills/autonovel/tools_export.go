package autonovel

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/iconidentify/chonkskill/pkg/anthropic"
	"github.com/iconidentify/chonkskill/pkg/project"
	"github.com/iconidentify/chonkskill/pkg/skill"
	"github.com/iconidentify/chonkskill/pkg/typeset"
)

func registerExportTools(s *skill.Skill, rt *runtime) {
	skill.AddTool(s, "build_arc_summary",
		"Build arc_summary.md from chapters. Per chapter: first 150 words, last 150 words, top 3 dialogue lines, and a 3-sentence LLM summary. Used by reader_panel.",
		func(ctx context.Context, args BuildArcSummaryArgs) (string, error) {
			p := project.New(args.ProjectDir)
			if err := buildArcSummaryForProject(p, rt); err != nil {
				return "", err
			}
			summary, _ := p.ArcSummary()
			return fmt.Sprintf("arc_summary.md built (%d words)", anthropic.CountWords(summary)), nil
		})

	skill.AddTool(s, "build_outline",
		"Rebuild outline.md from actual chapters (post-revision). One LLM call per chapter extracts beats, plants, payoffs, and emotional arc. Includes foreshadowing ledger.",
		func(ctx context.Context, args BuildOutlineArgs) (string, error) {
			p := project.New(args.ProjectDir)
			if err := rebuildOutline(p, rt); err != nil {
				return "", err
			}
			outline, _ := p.Outline()
			return fmt.Sprintf("outline.md rebuilt (%d words)", anthropic.CountWords(outline)), nil
		})

	skill.AddTool(s, "prepare_pdf",
		"Prepare all LaTeX files for professional PDF generation. Converts chapters to LaTeX with drop caps, scene breaks, and ornaments. Writes typeset/novel.tex and typeset/chapters_content.tex. The agent must then run the tectonic command on the sandbox.",
		func(ctx context.Context, args PreparePDFArgs) (string, error) {
			p := project.New(args.ProjectDir)
			title := extractTitle(p)
			author := args.Author
			if author == "" {
				author = "Author"
			}

			opts := typeset.Options{
				Title:    title,
				Author:   author,
				DropCaps: true,
			}
			if err := typeset.PreparePDF(args.ProjectDir, opts); err != nil {
				return "", err
			}

			cmd := typeset.PDFCommand(args.ProjectDir, false)
			return fmt.Sprintf("PDF files prepared in typeset/.\n\nRun this command to compile:\n\n```\n%s\n```\n\nOutput: typeset/novel.pdf", cmd), nil
		})

	skill.AddTool(s, "prepare_epub",
		"Prepare metadata and CSS files for EPUB generation via pandoc. Writes typeset/epub_metadata.yaml and typeset/epub_style.css. The agent must then run the pandoc command on the sandbox.",
		func(ctx context.Context, args PrepareEPUBArgs) (string, error) {
			p := project.New(args.ProjectDir)
			title := extractTitle(p)
			author := args.Author
			if author == "" {
				author = "Author"
			}

			opts := typeset.Options{Title: title, Author: author}
			if err := typeset.PrepareEPUB(args.ProjectDir, opts); err != nil {
				return "", err
			}

			cmd := typeset.EPUBCommand(args.ProjectDir)
			return fmt.Sprintf("EPUB files prepared in typeset/.\n\nRun this command to compile:\n\n```\n%s\n```\n\nOutput: book.epub", cmd), nil
		})
}

// buildArcSummaryForProject generates arc_summary.md.
func buildArcSummaryForProject(p *project.Project, rt *runtime) error {
	chapters, err := p.LoadAllChapters()
	if err != nil || len(chapters) == 0 {
		return fmt.Errorf("no chapters found")
	}

	chNums, _ := p.ChapterNumbers()
	var sb strings.Builder

	// Preamble from outline title.
	title := extractTitle(p)
	sb.WriteString(fmt.Sprintf("# Arc Summary: %s\n\n", title))

	for _, n := range chNums {
		text := chapters[n]
		words := strings.Fields(text)

		// First 150 words.
		opening := strings.Join(firstN(words, 150), " ")
		// Last 150 words.
		closing := strings.Join(lastN(words, 150), " ")
		// Top 3 dialogue lines by length.
		dialogueLines := extractDialogue(text, 3)

		// LLM summary.
		summaryPrompt := fmt.Sprintf(`Summarize this chapter in exactly 3 sentences. Be precise about plot events, character actions, and revelations. Do not editorialize.

%s`, truncateForContext(text, 8000))

		resp, err := rt.client.Message(anthropic.Request{
			Model:       rt.writerModel,
			System:      "You summarize novel chapters precisely in exactly 3 sentences.",
			Prompt:      summaryPrompt,
			MaxTokens:   500,
			Temperature: 0.1,
		})
		summary := ""
		if err == nil {
			summary = resp.Text
		}

		chTitle := extractChapterTitle(text)
		sb.WriteString(fmt.Sprintf("## Chapter %d: %s\n\n", n, chTitle))
		sb.WriteString(fmt.Sprintf("**Summary**: %s\n\n", summary))
		sb.WriteString(fmt.Sprintf("**Opening** (%d words): %s\n\n", min(150, len(words)), opening))
		sb.WriteString(fmt.Sprintf("**Closing** (%d words): %s\n\n", min(150, len(words)), closing))
		if len(dialogueLines) > 0 {
			sb.WriteString("**Key Dialogue**:\n")
			for _, line := range dialogueLines {
				sb.WriteString(fmt.Sprintf("- %s\n", line))
			}
			sb.WriteString("\n")
		}
		sb.WriteString("---\n\n")
	}

	return p.SaveArcSummary(sb.String())
}

// rebuildOutline generates outline.md from actual chapters.
func rebuildOutline(p *project.Project, rt *runtime) error {
	chapters, err := p.LoadAllChapters()
	if err != nil || len(chapters) == 0 {
		return fmt.Errorf("no chapters found")
	}

	chNums, _ := p.ChapterNumbers()
	var outlineEntries []string
	var allPlants []map[string]string
	var allPayoffs []map[string]string

	for _, n := range chNums {
		text := chapters[n]

		prompt := fmt.Sprintf(`Analyze this chapter and extract a structured outline entry.

%s

Return a JSON object:
{
  "title": "chapter title",
  "location": "primary location",
  "characters": ["names of characters who appear"],
  "summary": "2-3 sentence summary",
  "beats": ["beat 1", "beat 2", "beat 3"],
  "try_fail": "what the protagonist attempts and how it goes",
  "plants": ["foreshadowing elements planted"],
  "harvests": ["foreshadowing elements resolved from earlier"],
  "emotional_arc": "character's emotional state start -> end",
  "chapter_question": "the question this chapter raises or answers"
}`, truncateForContext(text, 8000))

		resp, err := rt.client.Message(anthropic.Request{
			Model:       rt.writerModel,
			System:      "You extract structured outline data from novel chapters. Respond only in JSON.",
			Prompt:      prompt,
			MaxTokens:   1500,
			Temperature: 0.1,
		})
		if err != nil {
			continue
		}

		parsed, err := anthropic.ParseJSON(resp.Text)
		if err != nil {
			continue
		}

		title := extractStringFromMap(parsed, "title")
		location := extractStringFromMap(parsed, "location")
		summary := extractStringFromMap(parsed, "summary")
		tryFail := extractStringFromMap(parsed, "try_fail")
		emotionalArc := extractStringFromMap(parsed, "emotional_arc")
		question := extractStringFromMap(parsed, "chapter_question")
		beats := extractStringSliceFromMap(parsed, "beats")
		characters := extractStringSliceFromMap(parsed, "characters")
		plants := extractStringSliceFromMap(parsed, "plants")
		harvests := extractStringSliceFromMap(parsed, "harvests")

		var entry strings.Builder
		entry.WriteString(fmt.Sprintf("### Ch %d: %s\n", n, title))
		entry.WriteString(fmt.Sprintf("- **Location**: %s\n", location))
		entry.WriteString(fmt.Sprintf("- **Characters**: %s\n", strings.Join(characters, ", ")))
		entry.WriteString(fmt.Sprintf("- **Summary**: %s\n", summary))
		entry.WriteString("- **Beats**:\n")
		for _, b := range beats {
			entry.WriteString(fmt.Sprintf("  - %s\n", b))
		}
		entry.WriteString(fmt.Sprintf("- **Try-Fail**: %s\n", tryFail))
		entry.WriteString(fmt.Sprintf("- **Plants**: %s\n", strings.Join(plants, "; ")))
		entry.WriteString(fmt.Sprintf("- **Payoffs**: %s\n", strings.Join(harvests, "; ")))
		entry.WriteString(fmt.Sprintf("- **Emotional Arc**: %s\n", emotionalArc))
		entry.WriteString(fmt.Sprintf("- **Chapter Question**: %s\n", question))

		outlineEntries = append(outlineEntries, entry.String())

		for _, plant := range plants {
			allPlants = append(allPlants, map[string]string{"thread": plant, "chapter": fmt.Sprintf("%d", n)})
		}
		for _, harvest := range harvests {
			allPayoffs = append(allPayoffs, map[string]string{"thread": harvest, "chapter": fmt.Sprintf("%d", n)})
		}
	}

	// Assemble outline.
	var sb strings.Builder
	title := extractTitle(p)
	sb.WriteString(fmt.Sprintf("# %s -- Outline (Rebuilt from Chapters)\n\n", title))
	for _, entry := range outlineEntries {
		sb.WriteString(entry + "\n")
	}

	// Foreshadowing ledger.
	sb.WriteString("## Foreshadowing Ledger\n\n")
	sb.WriteString("| Thread | Planted (Ch) | Payoff (Ch) |\n")
	sb.WriteString("|--------|-------------|-------------|\n")
	for _, plant := range allPlants {
		payoffCh := "?"
		for _, payoff := range allPayoffs {
			if fuzzyMatch(plant["thread"], payoff["thread"]) {
				payoffCh = payoff["chapter"]
				break
			}
		}
		sb.WriteString(fmt.Sprintf("| %s | %s | %s |\n", plant["thread"], plant["chapter"], payoffCh))
	}

	return p.SaveOutline(sb.String())
}

// Helpers.

func firstN(words []string, n int) []string {
	if len(words) <= n {
		return words
	}
	return words[:n]
}

func lastN(words []string, n int) []string {
	if len(words) <= n {
		return words
	}
	return words[len(words)-n:]
}

var dialogueExtractRe = regexp.MustCompile(`[""` + "\u201C" + `]([^""` + "\u201C\u201D" + `]+)[""` + "\u201D" + `]`)

func extractDialogue(text string, maxLines int) []string {
	matches := dialogueExtractRe.FindAllStringSubmatch(text, -1)
	// Sort by length descending.
	type dlg struct {
		text string
		len  int
	}
	var lines []dlg
	for _, m := range matches {
		if len(m) > 1 && len(strings.Fields(m[1])) > 5 {
			lines = append(lines, dlg{m[0], len(m[1])})
		}
	}
	// Simple sort.
	for i := 0; i < len(lines); i++ {
		for j := i + 1; j < len(lines); j++ {
			if lines[j].len > lines[i].len {
				lines[i], lines[j] = lines[j], lines[i]
			}
		}
	}
	var result []string
	for i, l := range lines {
		if i >= maxLines {
			break
		}
		result = append(result, l.text)
	}
	return result
}

func extractChapterTitle(text string) string {
	lines := strings.SplitN(text, "\n", 5)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") {
			return strings.TrimSpace(strings.TrimLeft(line, "#"))
		}
	}
	return "Untitled"
}

func fuzzyMatch(a, b string) bool {
	a = strings.ToLower(a)
	b = strings.ToLower(b)
	// Check if key words overlap.
	aWords := strings.Fields(a)
	bWords := strings.Fields(b)
	matches := 0
	for _, aw := range aWords {
		if len(aw) < 4 {
			continue
		}
		for _, bw := range bWords {
			if aw == bw {
				matches++
				break
			}
		}
	}
	return matches >= 2 || strings.Contains(a, b) || strings.Contains(b, a)
}

func extractStringFromMap(m map[string]any, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func extractStringSliceFromMap(m map[string]any, key string) []string {
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
