package autonovel

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/iconidentify/chonkskill/pkg/skill"
	"github.com/iconidentify/chonkskill/pkg/anthropic"
	"github.com/iconidentify/chonkskill/pkg/falai"
	"github.com/iconidentify/chonkskill/pkg/project"
)

func registerArtTools(s *skill.Skill, rt *runtime) {
	skill.AddTool(s, "gen_art_style",
		"Derive a visual style from the novel's world and voice. Saves art/visual_style.json with art style, color palette, texture, mood, reference artists, and concepts for cover/ornament/map/scene-break.",
		func(ctx context.Context, args GenArtStyleArgs) (string, error) {
			if rt.falClient == nil {
				return "", fmt.Errorf("FAL_KEY is required for art generation")
			}
			p := project.New(args.ProjectDir)

			world, _ := p.World()
			voice, _ := p.Voice()

			prompt := fmt.Sprintf(`Derive a visual art direction from this novel's world and voice.

## World Bible (excerpt)
%s

## Voice Guidelines (excerpt)
%s

Return a JSON object:
{
  "art_style": "e.g. art nouveau, ink wash, woodcut, digital painting",
  "color_palette": ["color1", "color2", "color3", "color4", "color5"],
  "texture": "e.g. rough paper, smooth vellum, aged parchment",
  "mood": "e.g. brooding, luminous, austere, warm",
  "reference_artists": ["artist1", "artist2", "artist3"],
  "cover_concept": "visual concept for the cover",
  "ornament_concept": "concept for chapter ornaments",
  "scene_break_concept": "concept for scene break decorations",
  "map_concept": "concept for a world map"
}`, truncateForContext(world, 4000), truncateForContext(voice, 2000))

			resp, err := rt.client.Message(anthropic.Request{
				Model:       rt.writerModel,
				System:      "You are a book designer who derives visual style from prose. Respond only in JSON.",
				Prompt:      prompt,
				MaxTokens:   1500,
				Temperature: 0.3,
			})
			if err != nil {
				return "", err
			}

			parsed, err := anthropic.ParseJSON(resp.Text)
			if err != nil {
				return "", fmt.Errorf("parsing style response: %w", err)
			}

			styleJSON, _ := json.MarshalIndent(parsed, "", "  ")
			if err := p.SaveFile("art/visual_style.json", string(styleJSON)); err != nil {
				return "", err
			}

			return fmt.Sprintf("Visual style saved to art/visual_style.json:\n%s", string(styleJSON)), nil
		})

	skill.AddTool(s, "gen_art",
		"Generate art assets using fal.ai. Supports cover, ornament (per-chapter), map, and scene_break types. Uses the visual style from gen_art_style if available.",
		func(ctx context.Context, args GenArtArgs) (string, error) {
			if rt.falClient == nil {
				return "", fmt.Errorf("FAL_KEY is required for art generation")
			}
			p := project.New(args.ProjectDir)

			// Load visual style.
			var style map[string]any
			styleJSON, _ := p.LoadFile("art/visual_style.json")
			if styleJSON != "" {
				json.Unmarshal([]byte(styleJSON), &style)
			}

			prompt := args.Prompt
			if prompt == "" {
				prompt = buildArtPrompt(args.ArtType, args.Chapter, style)
			}

			var resolution, aspectRatio string
			switch args.ArtType {
			case "cover":
				resolution = "1024x1536"
				aspectRatio = "2:3"
			case "ornament", "scene_break":
				resolution = "512x512"
				aspectRatio = "1:1"
			case "map":
				resolution = "1536x1024"
				aspectRatio = "3:2"
			default:
				return "", fmt.Errorf("unknown art_type: %s (use cover, ornament, map, scene_break)", args.ArtType)
			}

			result, err := rt.falClient.Generate(falai.GenerateParams{
				Prompt:      prompt,
				Resolution:  resolution,
				AspectRatio: aspectRatio,
			})
			if err != nil {
				return "", fmt.Errorf("generation failed: %w", err)
			}

			if result.ImageURL == "" {
				return "", fmt.Errorf("no image URL in response")
			}

			// Download the image.
			var destPath string
			switch args.ArtType {
			case "cover":
				destPath = p.Dir + "/art/cover.png"
			case "ornament":
				destPath = p.Dir + fmt.Sprintf("/art/ornament_ch%02d.png", args.Chapter)
			case "map":
				destPath = p.Dir + "/art/map.png"
			case "scene_break":
				destPath = p.Dir + "/art/scene_break.png"
			}

			bytes, err := rt.falClient.DownloadImage(result.ImageURL, destPath)
			if err != nil {
				return "", fmt.Errorf("download failed: %w", err)
			}

			return fmt.Sprintf("Art generated: %s (%d bytes)\nPrompt: %s\nURL: %s",
				filepath.Base(destPath), bytes, prompt, result.ImageURL), nil
		})
}

func buildArtPrompt(artType string, chapter int, style map[string]any) string {
	artStyle := extractString(style, "art_style")
	palette := ""
	if colors, ok := style["color_palette"].([]any); ok {
		var strs []string
		for _, c := range colors {
			if s, ok := c.(string); ok {
				strs = append(strs, s)
			}
		}
		palette = fmt.Sprintf(", color palette: %s", join(strs))
	}
	mood := extractString(style, "mood")

	switch artType {
	case "cover":
		concept := extractString(style, "cover_concept")
		return fmt.Sprintf("Book cover, %s style, %s%s, %s, no text, high detail, dramatic lighting", artStyle, concept, palette, mood)
	case "ornament":
		concept := extractString(style, "ornament_concept")
		return fmt.Sprintf("Chapter ornament for chapter %d, %s style, %s%s, black and white, decorative, symmetrical", chapter, artStyle, concept, palette)
	case "map":
		concept := extractString(style, "map_concept")
		return fmt.Sprintf("Fantasy world map, %s style, %s%s, parchment texture, labeled regions, compass rose", artStyle, concept, palette)
	case "scene_break":
		concept := extractString(style, "scene_break_concept")
		return fmt.Sprintf("Scene break decoration, %s style, %s%s, small, centered, minimal", artStyle, concept, palette)
	}
	return "fantasy art"
}

func extractString(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func join(strs []string) string {
	result := ""
	for i, s := range strs {
		if i > 0 {
			result += ", "
		}
		result += s
	}
	return result
}
