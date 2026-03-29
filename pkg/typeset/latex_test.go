package typeset

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEscapeLatex(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"hello", "hello"},
		{"a & b", `a \& b`},
		{"100%", `100\%`},
		{"$5", `\$5`},
		{"#1", `\#1`},
		{"a_b", `a\_b`},
	}
	for _, tt := range tests {
		got := EscapeLatex(tt.in)
		if got != tt.want {
			t.Errorf("EscapeLatex(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestMarkdownToLatex_SceneBreaks(t *testing.T) {
	for _, sep := range []string{"---", "***", "* * *"} {
		result := MarkdownToLatex("paragraph one\n\n" + sep + "\n\nparagraph two")
		if !strings.Contains(result, `\scenebreak`) {
			t.Errorf("expected \\scenebreak for %q, got: %s", sep, result)
		}
	}
}

func TestMarkdownToLatex_Italics(t *testing.T) {
	result := MarkdownToLatex("She said *nothing* at all.")
	if !strings.Contains(result, `\emph{nothing}`) {
		t.Errorf("expected \\emph{nothing}, got: %s", result)
	}
}

func TestMarkdownToLatex_CurlyQuotes(t *testing.T) {
	result := MarkdownToLatex("\u201CHello,\u201D she said.")
	if !strings.Contains(result, "``Hello,''") {
		t.Errorf("expected TeX quotes, got: %s", result)
	}
}

func TestMarkdownToLatex_EmDash(t *testing.T) {
	result := MarkdownToLatex("word\u2014word")
	if !strings.Contains(result, "word---word") {
		t.Errorf("expected ---, got: %s", result)
	}
}

func TestMarkdownToLatex_Ellipsis(t *testing.T) {
	result := MarkdownToLatex("wait\u2026")
	if !strings.Contains(result, `\ldots{}`) {
		t.Errorf("expected \\ldots{}, got: %s", result)
	}
}

func TestMakeDropCap(t *testing.T) {
	result := MakeDropCap("The bell rang across the valley.")
	if !strings.Contains(result, `\lettrine`) {
		t.Errorf("expected \\lettrine, got: %s", result)
	}
	if !strings.Contains(result, `{T}{he}`) {
		t.Errorf("expected {T}{he}, got: %s", result)
	}
}

func TestMakeDropCap_Empty(t *testing.T) {
	result := MakeDropCap("")
	if result != "" {
		t.Errorf("expected empty, got: %q", result)
	}
}

func TestConvertChapter(t *testing.T) {
	body := "The bell rang.\n\n---\n\nShe looked up."
	result := ConvertChapter(1, "The Tower", body, false, false, true)

	if !strings.Contains(result, `\chapter{The Tower}`) {
		t.Error("missing chapter command")
	}
	if !strings.Contains(result, `\scenebreak`) {
		t.Error("missing scene break")
	}
	if !strings.Contains(result, `\lettrine`) {
		t.Error("missing drop cap")
	}
}

func TestConvertChapter_NoDropCaps(t *testing.T) {
	result := ConvertChapter(1, "Title", "Some text here.", false, false, false)
	if strings.Contains(result, `\lettrine`) {
		t.Error("should not have drop cap when disabled")
	}
}

func TestConvertChapter_WithArt(t *testing.T) {
	result := ConvertChapter(3, "Title", "Text", true, true, false)
	if !strings.Contains(result, `\chapterart{ornament_ch03.png}`) {
		t.Error("missing ornament")
	}
	if !strings.Contains(result, `\chapterillustration{ch03.png}`) {
		t.Error("missing illustration")
	}
}

func TestBuildChaptersTeX(t *testing.T) {
	dir := t.TempDir()
	chapDir := filepath.Join(dir, "chapters")
	os.MkdirAll(chapDir, 0o755)

	os.WriteFile(filepath.Join(chapDir, "ch_01.md"), []byte("# Chapter 1: The Start\n\nIt began."), 0o644)
	os.WriteFile(filepath.Join(chapDir, "ch_02.md"), []byte("# Chapter 2: The Middle\n\nIt continued."), 0o644)

	content, err := BuildChaptersTeX(dir, true)
	if err != nil {
		t.Fatalf("BuildChaptersTeX failed: %v", err)
	}
	if !strings.Contains(content, `\chapter{The Start}`) {
		t.Error("missing chapter 1")
	}
	if !strings.Contains(content, `\chapter{The Middle}`) {
		t.Error("missing chapter 2")
	}
}

func TestRenderNovelTeX(t *testing.T) {
	result := RenderNovelTeX("My Novel", "Jane Author")
	if !strings.Contains(result, "My Novel") {
		t.Error("missing title")
	}
	if !strings.Contains(result, "Jane Author") {
		t.Error("missing author")
	}
	if !strings.Contains(result, `\documentclass`) {
		t.Error("missing document class")
	}
}

func TestRenderKidsBookTeX(t *testing.T) {
	result := RenderKidsBookTeX("Kids Book", "Author", 3)
	if !strings.Contains(result, "12pt") {
		t.Error("grade 3 should use 12pt")
	}

	result5 := RenderKidsBookTeX("Kids Book", "Author", 5)
	if !strings.Contains(result5, "11pt") {
		t.Error("grade 5 should use 11pt")
	}
}

func TestRenderEPUBMetadata(t *testing.T) {
	result := RenderEPUBMetadata("Title", "Author", true)
	if !strings.Contains(result, "cover-image") {
		t.Error("missing cover-image")
	}

	noCover := RenderEPUBMetadata("Title", "Author", false)
	if strings.Contains(noCover, "cover-image") {
		t.Error("should not have cover-image when hasCover=false")
	}
}

func TestPreparePDF(t *testing.T) {
	dir := t.TempDir()
	chapDir := filepath.Join(dir, "chapters")
	os.MkdirAll(chapDir, 0o755)
	os.WriteFile(filepath.Join(chapDir, "ch_01.md"), []byte("# Chapter 1: Test\n\nContent here."), 0o644)

	err := PreparePDF(dir, Options{Title: "Test", Author: "Me", DropCaps: true})
	if err != nil {
		t.Fatalf("PreparePDF failed: %v", err)
	}

	// Check files were written.
	if _, err := os.Stat(filepath.Join(dir, "typeset", "novel.tex")); err != nil {
		t.Error("novel.tex not created")
	}
	if _, err := os.Stat(filepath.Join(dir, "typeset", "chapters_content.tex")); err != nil {
		t.Error("chapters_content.tex not created")
	}
}

func TestPrepareEPUB(t *testing.T) {
	dir := t.TempDir()

	err := PrepareEPUB(dir, Options{Title: "Test", Author: "Me"})
	if err != nil {
		t.Fatalf("PrepareEPUB failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "typeset", "epub_metadata.yaml")); err != nil {
		t.Error("epub_metadata.yaml not created")
	}
	if _, err := os.Stat(filepath.Join(dir, "typeset", "epub_style.css")); err != nil {
		t.Error("epub_style.css not created")
	}
}

func TestPDFCommand(t *testing.T) {
	cmd := PDFCommand("/tmp/novel", false)
	if !strings.Contains(cmd, "tectonic") {
		t.Error("missing tectonic")
	}
	if !strings.Contains(cmd, "novel.tex") {
		t.Error("missing novel.tex")
	}

	kidsCmd := PDFCommand("/tmp/novel", true)
	if !strings.Contains(kidsCmd, "kidsbook.tex") {
		t.Error("missing kidsbook.tex for kids mode")
	}
}
