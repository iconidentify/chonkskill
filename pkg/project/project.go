// Package project manages the novel project directory structure and state.
// It provides typed access to all project files (seed, voice, world, characters,
// outline, canon, chapters, briefs, eval logs, edit logs) and the pipeline state.
package project

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// State tracks pipeline progress. Serialized to state.json.
type State struct {
	Phase          string   `json:"phase"`
	CurrentFocus   string   `json:"current_focus,omitempty"`
	Iteration      int      `json:"iteration"`
	FoundationScore float64 `json:"foundation_score"`
	LoreScore      float64  `json:"lore_score"`
	ChaptersDrafted int     `json:"chapters_drafted"`
	ChaptersTotal  int      `json:"chapters_total"`
	NovelScore     float64  `json:"novel_score"`
	RevisionCycle  int      `json:"revision_cycle"`
	Debts          []string `json:"debts"`
}

// DefaultState returns a fresh pipeline state.
func DefaultState() State {
	return State{
		Phase: "foundation",
		Debts: []string{},
	}
}

// ResultEntry is one row of results.tsv.
type ResultEntry struct {
	Commit      string
	Phase       string
	Score       float64
	WordCount   int
	Status      string
	Description string
}

// Project represents a novel project directory.
type Project struct {
	Dir string
}

// New creates a Project for the given directory.
func New(dir string) *Project {
	return &Project{Dir: dir}
}

// Init creates the project directory structure and initial files.
func (p *Project) Init() error {
	dirs := []string{
		"chapters", "briefs", "eval_logs", "edit_logs", "art", "audiobook/scripts", "audiobook/chapters",
	}
	for _, d := range dirs {
		if err := os.MkdirAll(filepath.Join(p.Dir, d), 0o755); err != nil {
			return fmt.Errorf("creating directory %s: %w", d, err)
		}
	}

	state := DefaultState()
	if err := p.SaveState(state); err != nil {
		return fmt.Errorf("saving initial state: %w", err)
	}

	// Write results.tsv header if it doesn't exist.
	resultsPath := filepath.Join(p.Dir, "results.tsv")
	if _, err := os.Stat(resultsPath); os.IsNotExist(err) {
		if err := os.WriteFile(resultsPath, []byte("commit\tphase\tscore\tword_count\tstatus\tdescription\n"), 0o644); err != nil {
			return fmt.Errorf("writing results.tsv: %w", err)
		}
	}

	return nil
}

// path returns the full path for a relative project path.
func (p *Project) path(rel string) string {
	return filepath.Join(p.Dir, rel)
}

// LoadState reads state.json or returns default state if missing.
func (p *Project) LoadState() (State, error) {
	data, err := os.ReadFile(p.path("state.json"))
	if os.IsNotExist(err) {
		return DefaultState(), nil
	}
	if err != nil {
		return State{}, fmt.Errorf("reading state.json: %w", err)
	}
	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return State{}, fmt.Errorf("parsing state.json: %w", err)
	}
	return s, nil
}

// SaveState writes state.json.
func (p *Project) SaveState(s State) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling state: %w", err)
	}
	return os.WriteFile(p.path("state.json"), append(data, '\n'), 0o644)
}

// LogResult appends a row to results.tsv.
func (p *Project) LogResult(entry ResultEntry) error {
	f, err := os.OpenFile(p.path("results.tsv"), os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = fmt.Fprintf(f, "%s\t%s\t%.2f\t%d\t%s\t%s\n",
		entry.Commit, entry.Phase, entry.Score, entry.WordCount, entry.Status, entry.Description)
	return err
}

// File I/O for layer files.

func (p *Project) LoadFile(name string) (string, error) {
	data, err := os.ReadFile(p.path(name))
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (p *Project) SaveFile(name, content string) error {
	dir := filepath.Dir(p.path(name))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(p.path(name), []byte(content), 0o644)
}

func (p *Project) Seed() (string, error)       { return p.LoadFile("seed.txt") }
func (p *Project) Voice() (string, error)      { return p.LoadFile("voice.md") }
func (p *Project) World() (string, error)      { return p.LoadFile("world.md") }
func (p *Project) Characters() (string, error) { return p.LoadFile("characters.md") }
func (p *Project) Outline() (string, error)    { return p.LoadFile("outline.md") }
func (p *Project) Canon() (string, error)      { return p.LoadFile("canon.md") }
func (p *Project) Mystery() (string, error)    { return p.LoadFile("MYSTERY.md") }
func (p *Project) ArcSummary() (string, error) { return p.LoadFile("arc_summary.md") }

func (p *Project) SaveSeed(c string) error       { return p.SaveFile("seed.txt", c) }
func (p *Project) SaveVoice(c string) error      { return p.SaveFile("voice.md", c) }
func (p *Project) SaveWorld(c string) error      { return p.SaveFile("world.md", c) }
func (p *Project) SaveCharacters(c string) error { return p.SaveFile("characters.md", c) }
func (p *Project) SaveOutline(c string) error    { return p.SaveFile("outline.md", c) }
func (p *Project) SaveCanon(c string) error      { return p.SaveFile("canon.md", c) }
func (p *Project) SaveMystery(c string) error    { return p.SaveFile("MYSTERY.md", c) }
func (p *Project) SaveArcSummary(c string) error { return p.SaveFile("arc_summary.md", c) }

// Chapter I/O.

func (p *Project) ChapterPath(n int) string {
	return p.path(fmt.Sprintf("chapters/ch_%02d.md", n))
}

func (p *Project) LoadChapter(n int) (string, error) {
	return p.LoadFile(fmt.Sprintf("chapters/ch_%02d.md", n))
}

func (p *Project) SaveChapter(n int, content string) error {
	return p.SaveFile(fmt.Sprintf("chapters/ch_%02d.md", n), content)
}

// ChapterNumbers returns the sorted list of chapter numbers that exist.
func (p *Project) ChapterNumbers() ([]int, error) {
	matches, err := filepath.Glob(p.path("chapters/ch_*.md"))
	if err != nil {
		return nil, err
	}
	re := regexp.MustCompile(`ch_(\d+)\.md$`)
	var nums []int
	for _, m := range matches {
		if sub := re.FindStringSubmatch(filepath.Base(m)); sub != nil {
			if n, err := strconv.Atoi(sub[1]); err == nil {
				nums = append(nums, n)
			}
		}
	}
	sort.Ints(nums)
	return nums, nil
}

// LoadAllChapters returns all chapters keyed by number.
func (p *Project) LoadAllChapters() (map[int]string, error) {
	nums, err := p.ChapterNumbers()
	if err != nil {
		return nil, err
	}
	chapters := make(map[int]string, len(nums))
	for _, n := range nums {
		text, err := p.LoadChapter(n)
		if err != nil {
			return nil, fmt.Errorf("loading chapter %d: %w", n, err)
		}
		chapters[n] = text
	}
	return chapters, nil
}

// CountAllWords sums word counts across all chapters.
func (p *Project) CountAllWords() (int, error) {
	chapters, err := p.LoadAllChapters()
	if err != nil {
		return 0, err
	}
	total := 0
	for _, text := range chapters {
		total += len(strings.Fields(text))
	}
	return total, nil
}

// GetTotalChapters infers the total chapter count from the outline or state.
func (p *Project) GetTotalChapters(state State) (int, error) {
	if state.ChaptersTotal > 0 {
		return state.ChaptersTotal, nil
	}
	outline, err := p.Outline()
	if err != nil || outline == "" {
		return 24, nil // default
	}
	re := regexp.MustCompile(`### Ch (\d+):`)
	matches := re.FindAllStringSubmatch(outline, -1)
	if len(matches) > 0 {
		last := matches[len(matches)-1]
		if n, err := strconv.Atoi(last[1]); err == nil {
			return n, nil
		}
	}
	return 24, nil
}

// ExtractChapterOutline extracts the outline entry for a specific chapter.
func (p *Project) ExtractChapterOutline(outlineText string, chapterNum int) string {
	pattern := fmt.Sprintf(`(?m)^### Ch %d:.*`, chapterNum)
	nextPattern := fmt.Sprintf(`(?m)^### Ch %d:`, chapterNum+1)

	startRe := regexp.MustCompile(pattern)
	loc := startRe.FindStringIndex(outlineText)
	if loc == nil {
		return ""
	}

	rest := outlineText[loc[0]:]
	endRe := regexp.MustCompile(nextPattern)
	endLoc := endRe.FindStringIndex(rest)
	if endLoc != nil {
		return strings.TrimSpace(rest[:endLoc[0]])
	}
	return strings.TrimSpace(rest)
}

// Brief I/O.

func (p *Project) SaveBrief(chapterNum int, briefType, content string) error {
	name := fmt.Sprintf("briefs/ch%02d_%s.md", chapterNum, briefType)
	return p.SaveFile(name, content)
}

func (p *Project) LoadBrief(path string) (string, error) {
	return p.LoadFile(path)
}

// Eval log I/O.

func (p *Project) SaveEvalLog(phase string, data map[string]any) (string, error) {
	ts := time.Now().Format("2006-01-02T15-04-05")
	name := fmt.Sprintf("eval_logs/%s_%s.json", ts, phase)
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return "", err
	}
	return name, p.SaveFile(name, string(b))
}

func (p *Project) LatestEvalLog(suffix string) (map[string]any, error) {
	matches, err := filepath.Glob(p.path("eval_logs/*_" + suffix + ".json"))
	if err != nil || len(matches) == 0 {
		return nil, nil
	}
	sort.Strings(matches)
	latest := matches[len(matches)-1]
	data, err := os.ReadFile(latest)
	if err != nil {
		return nil, err
	}
	var result map[string]any
	return result, json.Unmarshal(data, &result)
}

// Edit log I/O.

func (p *Project) SaveEditLog(name string, data any) error {
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return p.SaveFile("edit_logs/"+name, string(b))
}

func (p *Project) LoadEditLog(name string) ([]byte, error) {
	data, err := os.ReadFile(p.path("edit_logs/" + name))
	if os.IsNotExist(err) {
		return nil, nil
	}
	return data, err
}

// LastNChars returns the last n characters of text.
func LastNChars(text string, n int) string {
	runes := []rune(text)
	if len(runes) <= n {
		return text
	}
	return string(runes[len(runes)-n:])
}

// FirstNChars returns the first n characters of text.
func FirstNChars(text string, n int) string {
	runes := []rune(text)
	if len(runes) <= n {
		return text
	}
	return string(runes[:n])
}
