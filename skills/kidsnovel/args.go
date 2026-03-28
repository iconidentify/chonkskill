package kidsnovel

// Typed argument structs for each tool.

type EmptyArgs struct{}

// --- Project ---

type InitProjectArgs struct {
	ProjectDir string `json:"project_dir" jsonschema:"Absolute path to the book project directory"`
	Grade      int    `json:"grade" jsonschema:"Target reading level: 3, 4, 5, or 6"`
}

type GetStateArgs struct {
	ProjectDir string `json:"project_dir" jsonschema:"Path to the book project directory"`
}

// --- Creation (collaborative with kids) ---

type FromKidWritingArgs struct {
	ProjectDir string `json:"project_dir" jsonschema:"Path to the book project directory"`
	Writing    string `json:"writing" jsonschema:"The kid's writing, story idea, or piece of text to build from"`
	KidName    string `json:"kid_name,omitempty" jsonschema:"The kid's name (for dedication and co-author credit)"`
}

type FromIdeaArgs struct {
	ProjectDir string `json:"project_dir" jsonschema:"Path to the book project directory"`
	Idea       string `json:"idea" jsonschema:"A story idea described in any way -- a sentence, a paragraph, a conversation"`
	Genre      string `json:"genre,omitempty" jsonschema:"Genre preference: adventure, mystery, fantasy, funny, sci-fi, animal, school, sports"`
}

type GenerateSeedArgs struct {
	ProjectDir string `json:"project_dir" jsonschema:"Path to the book project directory"`
	Count      int    `json:"count,omitempty" jsonschema:"Number of seed concepts to generate, default 5"`
	Theme      string `json:"theme,omitempty" jsonschema:"Optional theme to explore: friendship, bravery, kindness, being different, family, etc."`
}

// --- Foundation ---

type GenWorldArgs struct {
	ProjectDir string `json:"project_dir" jsonschema:"Path to the book project directory"`
}

type GenCharactersArgs struct {
	ProjectDir string `json:"project_dir" jsonschema:"Path to the book project directory"`
}

type GenOutlineArgs struct {
	ProjectDir string `json:"project_dir" jsonschema:"Path to the book project directory"`
}

// --- Drafting ---

type DraftChapterArgs struct {
	ProjectDir string `json:"project_dir" jsonschema:"Path to the book project directory"`
	Chapter    int    `json:"chapter" jsonschema:"Chapter number to draft"`
}

type DraftAllArgs struct {
	ProjectDir string `json:"project_dir" jsonschema:"Path to the book project directory"`
}

// --- Evaluation ---

type ReadabilityCheckArgs struct {
	ProjectDir string `json:"project_dir,omitempty" jsonschema:"Path to the book project directory (provide this or text)"`
	Chapter    int    `json:"chapter,omitempty" jsonschema:"Chapter number to check"`
	Text       string `json:"text,omitempty" jsonschema:"Raw text to check (alternative to chapter)"`
	Grade      int    `json:"grade,omitempty" jsonschema:"Override target grade (uses project grade if omitted)"`
}

type EvaluateChapterArgs struct {
	ProjectDir string `json:"project_dir" jsonschema:"Path to the book project directory"`
	Chapter    int    `json:"chapter" jsonschema:"Chapter number to evaluate"`
}

type EvaluateBookArgs struct {
	ProjectDir string `json:"project_dir" jsonschema:"Path to the book project directory"`
}

// --- Revision ---

type SimplifyChapterArgs struct {
	ProjectDir string `json:"project_dir" jsonschema:"Path to the book project directory"`
	Chapter    int    `json:"chapter" jsonschema:"Chapter number to simplify"`
}

type ReviseChapterArgs struct {
	ProjectDir string `json:"project_dir" jsonschema:"Path to the book project directory"`
	Chapter    int    `json:"chapter" jsonschema:"Chapter number to revise"`
	Feedback   string `json:"feedback,omitempty" jsonschema:"Specific revision feedback (from evaluate or readability check)"`
}

// --- Export ---

type BuildBookArgs struct {
	ProjectDir string `json:"project_dir" jsonschema:"Path to the book project directory"`
}

type GenIllustrationArgs struct {
	ProjectDir string `json:"project_dir" jsonschema:"Path to the book project directory"`
	Chapter    int    `json:"chapter,omitempty" jsonschema:"Chapter number (for chapter illustration). Omit for cover."`
	Style      string `json:"style,omitempty" jsonschema:"Art style override: cartoon, watercolor, pencil, comic, whimsical"`
}

// --- Pipeline ---

type RunPipelineArgs struct {
	ProjectDir string `json:"project_dir" jsonschema:"Path to the book project directory"`
	Phase      string `json:"phase,omitempty" jsonschema:"Run a specific phase: foundation, drafting, revision, export. Default: all."`
}
