package typeset

import (
	"fmt"
	"os"
	"path/filepath"
)

// Options controls typesetting behavior.
type Options struct {
	Grade          int    // 0 for adult fiction, 3-6 for kids
	DropCaps       bool   // Enable drop caps on first paragraph
	KidsBook       bool   // Use kids book template
	Title          string
	Author         string
}

// PreparePDF writes all LaTeX files needed for tectonic compilation.
// Returns nil when files are ready. The agent runs tectonic on the sandbox.
func PreparePDF(projectDir string, opts Options) error {
	typesetDir := filepath.Join(projectDir, "typeset")
	if err := os.MkdirAll(typesetDir, 0o755); err != nil {
		return fmt.Errorf("creating typeset dir: %w", err)
	}

	// Build chapters_content.tex from chapter markdown files.
	chaptersTeX, err := BuildChaptersTeX(projectDir, opts.DropCaps)
	if err != nil {
		return fmt.Errorf("building chapters TeX: %w", err)
	}

	if err := os.WriteFile(filepath.Join(typesetDir, "chapters_content.tex"), []byte(chaptersTeX), 0o644); err != nil {
		return fmt.Errorf("writing chapters_content.tex: %w", err)
	}

	// Render the appropriate template.
	var templateTeX string
	var templateName string
	if opts.KidsBook {
		templateTeX = RenderKidsBookTeX(opts.Title, opts.Author, opts.Grade)
		templateName = "kidsbook.tex"
	} else {
		templateTeX = RenderNovelTeX(opts.Title, opts.Author)
		templateName = "novel.tex"
	}

	if err := os.WriteFile(filepath.Join(typesetDir, templateName), []byte(templateTeX), 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", templateName, err)
	}

	return nil
}

// PrepareEPUB writes metadata and CSS files needed for pandoc EPUB generation.
func PrepareEPUB(projectDir string, opts Options) error {
	typesetDir := filepath.Join(projectDir, "typeset")
	if err := os.MkdirAll(typesetDir, 0o755); err != nil {
		return fmt.Errorf("creating typeset dir: %w", err)
	}

	// Check for cover art.
	hasCover := fileExists(filepath.Join(projectDir, "art", "cover.png"))

	// Write metadata YAML.
	metadata := RenderEPUBMetadata(opts.Title, opts.Author, hasCover)
	if err := os.WriteFile(filepath.Join(typesetDir, "epub_metadata.yaml"), []byte(metadata), 0o644); err != nil {
		return fmt.Errorf("writing epub_metadata.yaml: %w", err)
	}

	// Write CSS.
	if err := os.WriteFile(filepath.Join(typesetDir, "epub_style.css"), []byte(EPUBStyleCSS()), 0o644); err != nil {
		return fmt.Errorf("writing epub_style.css: %w", err)
	}

	return nil
}

// PDFCommand returns the shell command to compile the PDF.
func PDFCommand(projectDir string, kidsBook bool) string {
	templateName := "novel.tex"
	if kidsBook {
		templateName = "kidsbook.tex"
	}
	return fmt.Sprintf("cd %q && tectonic typeset/%s", projectDir, templateName)
}

// EPUBCommand returns the shell command to generate the EPUB.
func EPUBCommand(projectDir string) string {
	coverArg := ""
	if fileExists(filepath.Join(projectDir, "art", "cover.png")) {
		coverArg = " --epub-cover-image=art/cover.png"
	}
	return fmt.Sprintf("cd %q && pandoc manuscript.md --metadata-file=typeset/epub_metadata.yaml --css=typeset/epub_style.css -o book.epub --toc --toc-depth=1%s",
		projectDir, coverArg)
}
