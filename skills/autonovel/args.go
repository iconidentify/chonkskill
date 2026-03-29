package autonovel

// Typed argument structs for each tool. JSON tags control the parameter names,
// jsonschema tags provide descriptions for the generated schema.

type EmptyArgs struct{}

// --- Pipeline ---

type InitProjectArgs struct {
	ProjectDir string `json:"project_dir" jsonschema:"Absolute path to the novel project directory"`
}

type GetStateArgs struct {
	ProjectDir string `json:"project_dir" jsonschema:"Path to the novel project directory"`
}

type RunPipelineArgs struct {
	ProjectDir string `json:"project_dir" jsonschema:"Path to the novel project directory"`
	Phase      string `json:"phase,omitempty" jsonschema:"Run a specific phase: foundation, drafting, revision, export. Default: all remaining"`
	FromScratch bool  `json:"from_scratch,omitempty" jsonschema:"Start over from scratch, resetting all state"`
	MaxCycles  int    `json:"max_cycles,omitempty" jsonschema:"Max revision cycles, default 6"`
}

// --- Foundation ---

type GenerateSeedArgs struct {
	ProjectDir string `json:"project_dir" jsonschema:"Path to the novel project directory"`
	Count      int    `json:"count,omitempty" jsonschema:"Number of seed concepts to generate, default 10"`
	Riff       string `json:"riff,omitempty" jsonschema:"An existing idea to riff variations on"`
}

type GenWorldArgs struct {
	ProjectDir string `json:"project_dir" jsonschema:"Path to the novel project directory"`
}

type GenCharactersArgs struct {
	ProjectDir string `json:"project_dir" jsonschema:"Path to the novel project directory"`
}

type GenOutlineArgs struct {
	ProjectDir string `json:"project_dir" jsonschema:"Path to the novel project directory"`
}

type GenCanonArgs struct {
	ProjectDir string `json:"project_dir" jsonschema:"Path to the novel project directory"`
}

// --- Drafting ---

type DraftChapterArgs struct {
	ProjectDir string `json:"project_dir" jsonschema:"Path to the novel project directory"`
	Chapter    int    `json:"chapter" jsonschema:"Chapter number to draft"`
}

// --- Evaluation ---

type EvaluateFoundationArgs struct {
	ProjectDir string `json:"project_dir" jsonschema:"Path to the novel project directory"`
}

type EvaluateChapterArgs struct {
	ProjectDir string `json:"project_dir" jsonschema:"Path to the novel project directory"`
	Chapter    int    `json:"chapter" jsonschema:"Chapter number to evaluate"`
}

type EvaluateFullArgs struct {
	ProjectDir string `json:"project_dir" jsonschema:"Path to the novel project directory"`
}

type SlopCheckArgs struct {
	ProjectDir string `json:"project_dir,omitempty" jsonschema:"Path to the novel project directory (provide this or text)"`
	Chapter    int    `json:"chapter,omitempty" jsonschema:"Chapter number to check"`
	Text       string `json:"text,omitempty" jsonschema:"Raw text to check (alternative to chapter)"`
}

type VoiceFingerprintArgs struct {
	ProjectDir string `json:"project_dir" jsonschema:"Path to the novel project directory"`
}

// --- Revision ---

type AdversarialEditArgs struct {
	ProjectDir string `json:"project_dir" jsonschema:"Path to the novel project directory"`
	Chapter    int    `json:"chapter" jsonschema:"Chapter number to edit. Use 0 for all chapters."`
}

type ApplyCutsArgs struct {
	ProjectDir string   `json:"project_dir" jsonschema:"Path to the novel project directory"`
	Chapter    int      `json:"chapter" jsonschema:"Chapter number. Use 0 for all chapters with cuts."`
	Types      []string `json:"types,omitempty" jsonschema:"Cut types to apply: FAT, REDUNDANT, OVER-EXPLAIN, TELL, STRUCTURAL, GENERIC. Default: all."`
	MinFatPct  float64  `json:"min_fat_pct,omitempty" jsonschema:"Only apply to chapters above this fat percentage"`
	DryRun     bool     `json:"dry_run,omitempty" jsonschema:"Preview changes without modifying files"`
}

type ReaderPanelArgs struct {
	ProjectDir string `json:"project_dir" jsonschema:"Path to the novel project directory"`
}

type GenBriefArgs struct {
	ProjectDir string `json:"project_dir" jsonschema:"Path to the novel project directory"`
	Chapter    int    `json:"chapter" jsonschema:"Chapter number to brief"`
	Source     string `json:"source,omitempty" jsonschema:"Brief source: panel, eval, cuts, or auto. Default: auto"`
}

type GenRevisionArgs struct {
	ProjectDir string `json:"project_dir" jsonschema:"Path to the novel project directory"`
	Chapter    int    `json:"chapter" jsonschema:"Chapter number to revise"`
	BriefPath  string `json:"brief_path" jsonschema:"Path to the revision brief file (relative to project dir)"`
}

type ReviewManuscriptArgs struct {
	ProjectDir string `json:"project_dir" jsonschema:"Path to the novel project directory"`
}

type CompareChaptersArgs struct {
	ProjectDir string `json:"project_dir" jsonschema:"Path to the novel project directory"`
	ChapterA   int    `json:"chapter_a,omitempty" jsonschema:"First chapter number (omit both for full tournament)"`
	ChapterB   int    `json:"chapter_b,omitempty" jsonschema:"Second chapter number"`
}

// --- Export ---

type BuildArcSummaryArgs struct {
	ProjectDir string `json:"project_dir" jsonschema:"Path to the novel project directory"`
}

type BuildOutlineArgs struct {
	ProjectDir string `json:"project_dir" jsonschema:"Path to the novel project directory"`
}

// --- Typesetting ---

type PreparePDFArgs struct {
	ProjectDir string `json:"project_dir" jsonschema:"Path to the novel project directory"`
	Author     string `json:"author,omitempty" jsonschema:"Author name for title page"`
}

type PrepareEPUBArgs struct {
	ProjectDir string `json:"project_dir" jsonschema:"Path to the novel project directory"`
	Author     string `json:"author,omitempty" jsonschema:"Author name for metadata"`
}

// --- Art ---

type GenArtStyleArgs struct {
	ProjectDir string `json:"project_dir" jsonschema:"Path to the novel project directory"`
}

type GenArtArgs struct {
	ProjectDir string `json:"project_dir" jsonschema:"Path to the novel project directory"`
	ArtType    string `json:"art_type" jsonschema:"Type of art: cover, ornament, map, scene_break"`
	Chapter    int    `json:"chapter,omitempty" jsonschema:"Chapter number (for ornaments)"`
	Prompt     string `json:"prompt,omitempty" jsonschema:"Custom prompt override"`
}

// --- Audiobook ---

type GenAudiobookScriptArgs struct {
	ProjectDir string `json:"project_dir" jsonschema:"Path to the novel project directory"`
	Chapter    int    `json:"chapter,omitempty" jsonschema:"Chapter number. Use 0 for all."`
}

type GenAudiobookArgs struct {
	ProjectDir string `json:"project_dir" jsonschema:"Path to the novel project directory"`
	StartCh    int    `json:"start_chapter,omitempty" jsonschema:"Starting chapter number"`
	EndCh      int    `json:"end_chapter,omitempty" jsonschema:"Ending chapter number"`
}
