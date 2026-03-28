package readability

import (
	"testing"
)

func TestCountSyllables(t *testing.T) {
	tests := []struct {
		word string
		want int
	}{
		{"the", 1},
		{"cat", 1},
		{"happy", 2},
		{"elephant", 3},
		{"beautiful", 3},
		{"a", 1},
		{"I", 1},
		{"walked", 1},
		{"running", 2},
		{"adventure", 3},
		{"understanding", 4},
		{"communication", 5},
	}
	for _, tt := range tests {
		got := CountSyllables(tt.word)
		if got != tt.want {
			t.Errorf("CountSyllables(%q) = %d, want %d", tt.word, got, tt.want)
		}
	}
}

func TestAnalyze_Grade3(t *testing.T) {
	text := `Sam ran to the park. He saw a big dog. The dog was brown. It had a red ball.
"Can I play?" Sam asked. The dog dropped the ball. Sam picked it up. He threw it far.
The dog ran fast. It got the ball. Sam laughed. This was fun.`

	a := Analyze(text, 3)
	if a.WordCount == 0 {
		t.Fatal("expected non-zero word count")
	}
	if a.FleschKincaid > 4.0 {
		t.Errorf("simple text should have low FK grade, got %.1f", a.FleschKincaid)
	}
	if a.AvgSentenceLen > 8 {
		t.Errorf("simple text should have short sentences, got %.1f", a.AvgSentenceLen)
	}
}

func TestAnalyze_TooHard(t *testing.T) {
	text := `The sophisticated archaeological expedition traversed the uncharted territories,
encountering unprecedented circumstances that necessitated comprehensive deliberation
among the multidisciplinary team of distinguished researchers.`

	a := Analyze(text, 3)
	if a.GradeFit != "too-hard" {
		t.Errorf("complex text should be too-hard for grade 3, got %q", a.GradeFit)
	}
	if len(a.Issues) == 0 {
		t.Error("expected issues for complex text at grade 3")
	}
}

func TestGradeConstraints(t *testing.T) {
	for grade := 3; grade <= 6; grade++ {
		constraints := GradeConstraints(grade)
		if constraints == "" {
			t.Errorf("empty constraints for grade %d", grade)
		}
	}
}

func TestFormatReport(t *testing.T) {
	a := Analyze("Sam went to the store. He bought some milk.", 3)
	report := FormatReport(a)
	if report == "" {
		t.Error("expected non-empty report")
	}
}

func TestAllGradeSpecs(t *testing.T) {
	for grade := 3; grade <= 6; grade++ {
		spec, ok := GradeSpecs[grade]
		if !ok {
			t.Errorf("missing spec for grade %d", grade)
			continue
		}
		if spec.ChapterWordTarget == 0 {
			t.Errorf("grade %d has zero chapter word target", grade)
		}
		if spec.BookWordMax <= spec.BookWordMin {
			t.Errorf("grade %d: book max %d <= min %d", grade, spec.BookWordMax, spec.BookWordMin)
		}
		if spec.FKMax <= spec.FKMin {
			t.Errorf("grade %d: FK max %.1f <= min %.1f", grade, spec.FKMax, spec.FKMin)
		}
	}
}
