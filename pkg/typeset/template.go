package typeset

import (
	_ "embed"
	"strings"
)

//go:embed templates/novel.tex
var novelTemplate string

//go:embed templates/kidsbook.tex
var kidsbookTemplate string

//go:embed templates/epub_style.css
var epubStyleCSS string

// RenderNovelTeX fills the novel.tex template with title and author.
func RenderNovelTeX(title, author string) string {
	t := novelTemplate
	t = strings.ReplaceAll(t, "{{TITLE}}", title)
	t = strings.ReplaceAll(t, "{{AUTHOR}}", author)
	return t
}

// RenderKidsBookTeX fills the kidsbook.tex template with title, author, and grade.
func RenderKidsBookTeX(title, author string, grade int) string {
	t := kidsbookTemplate

	// Font size based on grade.
	fontSize := "12pt"
	if grade >= 5 {
		fontSize = "11pt"
	}

	t = strings.ReplaceAll(t, "{{FONTSIZE}}", fontSize)
	t = strings.ReplaceAll(t, "{{TITLE}}", title)
	t = strings.ReplaceAll(t, "{{AUTHOR}}", author)
	return t
}

// RenderEPUBMetadata generates the pandoc EPUB metadata YAML.
func RenderEPUBMetadata(title, author string, hasCover bool) string {
	var sb strings.Builder
	sb.WriteString("---\n")
	sb.WriteString("title: \"" + escapeYAML(title) + "\"\n")
	sb.WriteString("author: \"" + escapeYAML(author) + "\"\n")
	sb.WriteString("lang: en\n")
	if hasCover {
		sb.WriteString("cover-image: art/cover.png\n")
	}
	sb.WriteString("css: typeset/epub_style.css\n")
	sb.WriteString("---\n")
	return sb.String()
}

// EPUBStyleCSS returns the embedded EPUB stylesheet.
func EPUBStyleCSS() string {
	return epubStyleCSS
}

func escapeYAML(s string) string {
	s = strings.ReplaceAll(s, `"`, `\"`)
	return s
}
