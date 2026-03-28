package autonovel

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/iconidentify/chonkskill/pkg/skill"
	"github.com/iconidentify/chonkskill/pkg/anthropic"
	elclient "github.com/iconidentify/chonkskill/skills/autonovel/internal/elevenlabs"
	"github.com/iconidentify/chonkskill/pkg/project"
)

func registerAudiobookTools(s *skill.Skill, rt *runtime) {
	skill.AddTool(s, "gen_audiobook_script",
		"Parse novel chapters into speaker-attributed audiobook scripts. Each segment gets a speaker tag and optional ElevenLabs delivery tags ([whisper], [firmly], [pause], etc.).",
		func(ctx context.Context, args GenAudiobookScriptArgs) (string, error) {
			p := project.New(args.ProjectDir)

			chapters := []int{args.Chapter}
			if args.Chapter == 0 {
				var err error
				chapters, err = p.ChapterNumbers()
				if err != nil {
					return "", err
				}
			}

			var results []string
			for _, ch := range chapters {
				text, _ := p.LoadChapter(ch)
				if text == "" {
					continue
				}

				script, err := parseChapterToScript(rt, ch, text)
				if err != nil {
					results = append(results, fmt.Sprintf("Ch %d: ERROR: %v", ch, err))
					continue
				}

				scriptJSON, _ := json.MarshalIndent(script, "", "  ")
				scriptPath := fmt.Sprintf("audiobook/scripts/ch%02d_script.json", ch)
				if err := p.SaveFile(scriptPath, string(scriptJSON)); err != nil {
					results = append(results, fmt.Sprintf("Ch %d: ERROR saving: %v", ch, err))
					continue
				}

				// Count speakers.
				speakers := make(map[string]int)
				for _, seg := range script.Segments {
					speakers[seg.Speaker]++
				}
				results = append(results, fmt.Sprintf("Ch %d: %d segments, %d speakers",
					ch, len(script.Segments), len(speakers)))
			}

			return strings.Join(results, "\n"), nil
		})

	skill.AddTool(s, "gen_audiobook",
		"Generate MP3 audio from parsed scripts using ElevenLabs Text to Dialogue API. Requires audiobook_voices.json mapping character names to voice IDs.",
		func(ctx context.Context, args GenAudiobookArgs) (string, error) {
			if rt.elClient == nil {
				return "", fmt.Errorf("ELEVENLABS_API_KEY is required for audiobook generation")
			}
			p := project.New(args.ProjectDir)

			// Load voice mappings.
			voicesJSON, err := p.LoadFile("audiobook_voices.json")
			if err != nil || voicesJSON == "" {
				return "", fmt.Errorf("audiobook_voices.json is required -- create a mapping of character names to ElevenLabs voice IDs")
			}
			var voiceMap map[string]string
			if err := json.Unmarshal([]byte(voicesJSON), &voiceMap); err != nil {
				return "", fmt.Errorf("invalid audiobook_voices.json: %w", err)
			}

			// Default voice for NARRATOR or unmapped characters.
			defaultVoice := voiceMap["NARRATOR"]
			if defaultVoice == "" {
				for _, v := range voiceMap {
					defaultVoice = v
					break
				}
			}

			chNums, _ := p.ChapterNumbers()
			if args.StartCh > 0 {
				var filtered []int
				for _, n := range chNums {
					if n >= args.StartCh && (args.EndCh == 0 || n <= args.EndCh) {
						filtered = append(filtered, n)
					}
				}
				chNums = filtered
			}

			var results []string
			var audioFiles []string
			for _, ch := range chNums {
				scriptJSON, _ := p.LoadFile(fmt.Sprintf("audiobook/scripts/ch%02d_script.json", ch))
				if scriptJSON == "" {
					results = append(results, fmt.Sprintf("Ch %d: no script found", ch))
					continue
				}

				var script audiobookScript
				if err := json.Unmarshal([]byte(scriptJSON), &script); err != nil {
					results = append(results, fmt.Sprintf("Ch %d: invalid script", ch))
					continue
				}

				// Convert to ElevenLabs segments.
				var segments []elclient.Segment
				for _, seg := range script.Segments {
					segments = append(segments, elclient.Segment{
						Speaker: seg.Speaker,
						Text:    seg.Text,
					})
				}

				chunks := elclient.ChunkSegments(segments, voiceMap, defaultVoice, 0)

				var allAudio []byte
				for i, chunk := range chunks {
					audio, err := rt.elClient.TextToDialogue(chunk)
					if err != nil {
						results = append(results, fmt.Sprintf("Ch %d chunk %d: ERROR: %v", ch, i, err))
						continue
					}
					allAudio = append(allAudio, audio...)
					// Rate limit pause.
					time.Sleep(time.Duration(elclient.PauseBetweenMs) * time.Millisecond)
				}

				if len(allAudio) > 0 {
					audioPath := filepath.Join(p.Dir, fmt.Sprintf("audiobook/chapters/ch_%02d.mp3", ch))
					if err := elclient.SaveAudio(allAudio, audioPath); err != nil {
						results = append(results, fmt.Sprintf("Ch %d: ERROR saving: %v", ch, err))
						continue
					}
					audioFiles = append(audioFiles, audioPath)
					results = append(results, fmt.Sprintf("Ch %d: %d bytes", ch, len(allAudio)))
				}
			}

			// Assemble full audiobook if we have multiple chapters.
			if len(audioFiles) > 1 {
				sort.Strings(audioFiles)
				fullPath := filepath.Join(p.Dir, "audiobook/full_audiobook.mp3")
				if err := elclient.ConcatAudioFiles(audioFiles, fullPath); err != nil {
					results = append(results, fmt.Sprintf("Assembly ERROR: %v", err))
				} else {
					fi, _ := os.Stat(fullPath)
					if fi != nil {
						results = append(results, fmt.Sprintf("Full audiobook: %d bytes", fi.Size()))
					}
				}
			}

			return strings.Join(results, "\n"), nil
		})
}

type audiobookScript struct {
	Chapter       int              `json:"chapter"`
	Title         string           `json:"title"`
	Segments      []scriptSegment  `json:"segments"`
	TotalSegments int              `json:"total_segments"`
	Speakers      []string         `json:"speakers"`
}

type scriptSegment struct {
	Speaker string `json:"speaker"`
	Text    string `json:"text"`
}

func parseChapterToScript(rt *runtime, chapterNum int, text string) (*audiobookScript, error) {
	prompt := fmt.Sprintf(`Parse this novel chapter into speaker-attributed segments for audiobook narration.

Rules:
1. Every text segment must have exactly one speaker.
2. NARRATOR handles all non-dialogue text.
3. Character names must be UPPERCASE.
4. Include ElevenLabs v3 delivery tags where appropriate:
   - Emotions: [happy] [sad] [angry] [excited] [fearful] [disgusted] [surprised]
   - Delivery: [whisper] [firmly] [gently] [urgently]
   - Reactions: [gasp] [sigh] [laugh] [groan]
   - Pacing: [slowly] [pause]
5. Keep segments 1-4 sentences. Break long narration into multiple NARRATOR segments.
6. Never put narration and dialogue in the same segment.

## Chapter %d
%s

Return a JSON object:
{
  "chapter": %d,
  "title": "chapter title",
  "segments": [{"speaker": "NARRATOR", "text": "[slowly] Chapter text..."}, ...],
  "total_segments": N,
  "speakers": ["NARRATOR", "CHARACTER1", ...]
}`, chapterNum, truncateForContext(text, 12000), chapterNum)

	resp, err := rt.client.Message(anthropic.Request{
		Model:       rt.writerModel,
		System:      "You parse novel chapters into speaker-attributed audiobook scripts. Respond only in JSON.",
		Prompt:      prompt,
		MaxTokens:   8000,
		Temperature: 0.1,
	})
	if err != nil {
		return nil, err
	}

	parsed, err := anthropic.ParseJSON(resp.Text)
	if err != nil {
		return nil, fmt.Errorf("parsing script response: %w", err)
	}

	script := &audiobookScript{
		Chapter: chapterNum,
		Title:   extractStringFromMap(parsed, "title"),
	}

	if segs, ok := parsed["segments"].([]any); ok {
		for _, s := range segs {
			if sm, ok := s.(map[string]any); ok {
				script.Segments = append(script.Segments, scriptSegment{
					Speaker: extractStringFromMap(sm, "speaker"),
					Text:    extractStringFromMap(sm, "text"),
				})
			}
		}
	}
	script.TotalSegments = len(script.Segments)

	speakerSet := make(map[string]bool)
	for _, seg := range script.Segments {
		speakerSet[seg.Speaker] = true
	}
	for sp := range speakerSet {
		script.Speakers = append(script.Speakers, sp)
	}

	return script, nil
}
