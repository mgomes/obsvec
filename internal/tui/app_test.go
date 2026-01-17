package tui

import (
	"strings"
	"testing"
)

func TestWrapText_ShortText(t *testing.T) {
	lines := wrapText("Hello world", 80, 3)

	if len(lines) != 1 {
		t.Errorf("expected 1 line, got %d", len(lines))
	}

	if lines[0] != "Hello world" {
		t.Errorf("expected 'Hello world', got '%s'", lines[0])
	}
}

func TestWrapText_LongText(t *testing.T) {
	text := "This is a longer piece of text that should wrap to multiple lines when displayed"
	lines := wrapText(text, 40, 3)

	if len(lines) < 2 {
		t.Errorf("expected multiple lines, got %d", len(lines))
	}

	for i, line := range lines {
		if len(line) > 40 {
			t.Errorf("line %d exceeds width: len=%d", i, len(line))
		}
	}
}

func TestWrapText_MaxLines(t *testing.T) {
	text := strings.Repeat("word ", 100)
	lines := wrapText(text, 40, 3)

	if len(lines) > 3 {
		t.Errorf("expected max 3 lines, got %d", len(lines))
	}

	// Last line should have ellipsis
	lastLine := lines[len(lines)-1]
	if !strings.HasSuffix(lastLine, "...") {
		t.Errorf("expected last line to end with '...', got '%s'", lastLine)
	}
}

func TestWrapText_NewlinesRemoved(t *testing.T) {
	text := "Line one\nLine two\nLine three"
	lines := wrapText(text, 80, 3)

	for _, line := range lines {
		if strings.Contains(line, "\n") {
			t.Error("expected newlines to be removed")
		}
	}
}

func TestWrapText_EmptyString(t *testing.T) {
	lines := wrapText("", 80, 3)

	if lines != nil {
		t.Errorf("expected nil for empty string, got %v", lines)
	}
}

func TestWrapText_WhitespaceCollapsed(t *testing.T) {
	text := "Multiple   spaces    here"
	lines := wrapText(text, 80, 3)

	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}

	if strings.Contains(lines[0], "  ") {
		t.Errorf("expected whitespace to be collapsed, got '%s'", lines[0])
	}
}

func TestTruncate_ShortString(t *testing.T) {
	result := truncate("Hello", 10)
	if result != "Hello" {
		t.Errorf("expected 'Hello', got '%s'", result)
	}
}

func TestTruncate_LongString(t *testing.T) {
	result := truncate("Hello World", 8)
	if result != "Hello..." {
		t.Errorf("expected 'Hello...', got '%s'", result)
	}
}

func TestTruncate_NewlinesReplaced(t *testing.T) {
	result := truncate("Hello\nWorld", 20)
	if strings.Contains(result, "\n") {
		t.Error("expected newlines to be replaced")
	}
}
