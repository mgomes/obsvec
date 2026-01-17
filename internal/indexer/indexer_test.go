package indexer

import (
	"strings"
	"testing"
)

func TestChunkMarkdown_SimpleDocument(t *testing.T) {
	content := `# Title

This is the introduction paragraph with some text.

## Section One

Content for section one goes here.

## Section Two

Content for section two goes here.
`

	chunks := chunkMarkdown(content)

	if len(chunks) != 3 {
		t.Errorf("expected 3 chunks, got %d", len(chunks))
	}

	if chunks[0].Heading != "Title" {
		t.Errorf("expected heading 'Title', got '%s'", chunks[0].Heading)
	}

	if chunks[1].Heading != "Title > Section One" {
		t.Errorf("expected heading 'Title > Section One', got '%s'", chunks[1].Heading)
	}

	if chunks[2].Heading != "Title > Section Two" {
		t.Errorf("expected heading 'Title > Section Two', got '%s'", chunks[2].Heading)
	}
}

func TestChunkMarkdown_NestedHeadings(t *testing.T) {
	content := `# Main

## Sub

### SubSub

Content here.

## Another Sub

More content.
`

	chunks := chunkMarkdown(content)

	// Find the chunk with SubSub heading
	var subsubChunk *Chunk
	for i := range chunks {
		if chunks[i].Heading == "Main > Sub > SubSub" {
			subsubChunk = &chunks[i]
			break
		}
	}

	if subsubChunk == nil {
		t.Error("expected to find chunk with nested heading 'Main > Sub > SubSub'")
	}

	// Check that heading stack resets properly
	var anotherSubChunk *Chunk
	for i := range chunks {
		if chunks[i].Heading == "Main > Another Sub" {
			anotherSubChunk = &chunks[i]
			break
		}
	}

	if anotherSubChunk == nil {
		t.Error("expected heading stack to reset to 'Main > Another Sub'")
	}
}

func TestChunkMarkdown_LongContent(t *testing.T) {
	// Create content longer than maxChunkTokens * avgCharsPerToken (500 * 4 = 2000 chars)
	// Use multiple lines since chunking happens per-line
	var lines []string
	for i := 0; i < 100; i++ {
		lines = append(lines, "This is a line of text that adds up to create a long document.")
	}
	longContent := strings.Join(lines, "\n")

	content := "# Title\n\n" + longContent

	chunks := chunkMarkdown(content)

	if len(chunks) < 2 {
		t.Errorf("expected long content to be split into multiple chunks, got %d (len=%d chars)", len(chunks), len(longContent))
	}
}

func TestChunkMarkdown_EmptyDocument(t *testing.T) {
	chunks := chunkMarkdown("")

	if len(chunks) != 0 {
		t.Errorf("expected 0 chunks for empty document, got %d", len(chunks))
	}
}

func TestChunkMarkdown_NoHeadings(t *testing.T) {
	content := `Just some plain text without any headings.

Another paragraph here.
`

	chunks := chunkMarkdown(content)

	if len(chunks) != 1 {
		t.Errorf("expected 1 chunk, got %d", len(chunks))
	}

	if chunks[0].Heading != "" {
		t.Errorf("expected empty heading, got '%s'", chunks[0].Heading)
	}
}

func TestChunkMarkdown_MinimumLength(t *testing.T) {
	content := `# Title

Hi
`

	chunks := chunkMarkdown(content)

	// "Hi" is less than 20 chars, should be filtered out
	if len(chunks) != 0 {
		t.Errorf("expected 0 chunks (content too short), got %d", len(chunks))
	}
}

func TestParseMarkdown_TitleWithH1(t *testing.T) {
	content := `# My Document Title

Some content here.
`

	title, _ := parseMarkdown(content, "fallback.md")

	if title != "My Document Title" {
		t.Errorf("expected 'My Document Title', got '%s'", title)
	}
}

func TestParseMarkdown_TitleNoH1(t *testing.T) {
	content := `Some content without a title.

## Section
`

	title, _ := parseMarkdown(content, "my-note.md")

	if title != "my-note" {
		t.Errorf("expected 'my-note', got '%s'", title)
	}
}

func TestParseMarkdown_TitleH1NotFirst(t *testing.T) {
	content := `Some preamble text.

# Actual Title

Content.
`

	title, _ := parseMarkdown(content, "fallback.md")

	// extractTitle finds first H1, even if not on first line
	if title != "Actual Title" {
		t.Errorf("expected 'Actual Title', got '%s'", title)
	}
}
