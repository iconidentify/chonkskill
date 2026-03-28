package slop

import (
	"testing"
)

func TestAnalyze_Clean(t *testing.T) {
	text := "The bell rang once across the valley. Birds scattered from the tower. A child looked up."
	score := Analyze(text)
	if score.SlopPenalty > 0 {
		t.Errorf("clean text should have zero slop penalty, got %.1f", score.SlopPenalty)
	}
	if score.Tier1Hits > 0 {
		t.Errorf("clean text should have zero tier1 hits, got %d", score.Tier1Hits)
	}
}

func TestAnalyze_BannedWords(t *testing.T) {
	text := "Let us delve into the comprehensive paradigm that leverages the holistic approach."
	score := Analyze(text)
	if score.Tier1Hits < 3 {
		t.Errorf("expected at least 3 tier1 hits, got %d", score.Tier1Hits)
	}
	if score.SlopPenalty == 0 {
		t.Error("expected non-zero slop penalty for banned words")
	}
}

func TestAnalyze_FictionAITells(t *testing.T) {
	text := "His eyes widened. A sense of dread washed over him. The silence stretched between them. Her heart pounded in her chest."
	score := Analyze(text)
	if score.FictionAITells < 3 {
		t.Errorf("expected at least 3 fiction AI tells, got %d", score.FictionAITells)
	}
}

func TestAnalyze_StructuralTics(t *testing.T) {
	text := "It wasn't the cold that bothered her. It was the silence. Not just the silence, but the way it pressed against everything."
	score := Analyze(text)
	if score.StructuralAITics < 1 {
		t.Errorf("expected at least 1 structural tic, got %d", score.StructuralAITics)
	}
}

func TestAnalyze_FillerPhrases(t *testing.T) {
	text := "It's worth noting that the tower stood at the edge of the city. In today's world, such structures are rare. At the end of the day, the bell must ring."
	score := Analyze(text)
	if score.Tier3Hits < 2 {
		t.Errorf("expected at least 2 tier3 hits, got %d", score.Tier3Hits)
	}
}

func TestAnalyze_TellingPatterns(t *testing.T) {
	text := `He felt a sudden surge of anger. "You don't understand," he said angrily. She felt a deep sense of dread.`
	score := Analyze(text)
	if score.TellingViolations < 1 {
		t.Errorf("expected at least 1 telling violation, got %d", score.TellingViolations)
	}
}

func TestAnalyze_PenaltyCap(t *testing.T) {
	// Max penalty is 10.0.
	text := "Let us delve and utilize and leverage and facilitate and endeavor. "
	text += "The aforementioned comprehensive paradigm was holistic and nuanced and multifaceted and intricate. "
	text += "It's worth noting that in today's world at the end of the day needless to say. "
	text += "His eyes widened. A sense of dread. Heart pounded in. Silence stretched. Darkness closed in. "
	text += "Not just fear, but terror. It wasn't courage. It was madness. And yet, everything changed. "
	text += "She felt a sudden surge of anger. He felt a deep sense of dread. "
	score := Analyze(text)
	if score.SlopPenalty > 10.0 {
		t.Errorf("penalty should be capped at 10.0, got %.1f", score.SlopPenalty)
	}
}

func TestCountWord(t *testing.T) {
	tests := []struct {
		text  string
		word  string
		count int
	}{
		{"the delve is here", "delve", 1},
		{"delve delve delve", "delve", 3},
		{"undelved", "delve", 0},
		{"they delved deep", "delve", 0},
		{"", "delve", 0},
	}
	for _, tt := range tests {
		got := countWord(tt.text, tt.word)
		if got != tt.count {
			t.Errorf("countWord(%q, %q) = %d, want %d", tt.text, tt.word, got, tt.count)
		}
	}
}
