package tui

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type SearchModel struct {
	query    string
	results  []SearchResult
	selected int
	error    string
	width    int
	height   int
	vaultDir string
}

func NewSearchModel(query, vaultDir string) SearchModel {
	return SearchModel{
		query:    query,
		vaultDir: vaultDir,
	}
}

func (m SearchModel) Init() tea.Cmd {
	return nil
}

func (m SearchModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit

		case "up", "k":
			if m.selected > 0 {
				m.selected--
			}

		case "down", "j":
			if m.selected < len(m.results)-1 {
				m.selected++
			}

		case "enter":
			if len(m.results) > 0 && m.selected < len(m.results) {
				result := m.results[m.selected]
				openInObsidian(m.vaultDir, result.Path)
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case SearchResultsMsg:
		m.results = msg.Results
		m.selected = 0

	case SearchErrorMsg:
		m.error = msg.Error
	}

	return m, nil
}

func (m SearchModel) View() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("ofind") + " ")
	b.WriteString(dimStyle.Render("\""+m.query+"\"") + "\n\n")

	if m.error != "" {
		b.WriteString(errorStyle.Render("Error: "+m.error) + "\n")
		return b.String()
	}

	if len(m.results) == 0 {
		b.WriteString(dimStyle.Render("No results found") + "\n")
		b.WriteString("\n" + helpStyle.Render("q quit"))
		return b.String()
	}

	for i, result := range m.results {
		isSelected := i == m.selected

		var line strings.Builder

		if isSelected {
			line.WriteString(selectedStyle.Render("> "))
		} else {
			line.WriteString("  ")
		}

		scoreStr := fmt.Sprintf("[%.2f]", result.Score)
		line.WriteString(scoreStyle.Render(scoreStr) + " ")

		line.WriteString(pathStyle.Render(result.Path))
		b.WriteString(line.String() + "\n")

		indent := "    "
		if result.Heading != "" {
			b.WriteString(indent + headingStyle.Render(result.Heading) + "\n")
		}

		snippetLines := wrapText(result.Snippet, 76, 3)
		for _, line := range snippetLines {
			b.WriteString(indent + snippetStyle.Render(line) + "\n")
		}
		b.WriteString("\n")
	}

	b.WriteString(helpStyle.Render("↑/↓ navigate  enter open in Obsidian  q quit"))

	return b.String()
}

func truncate(s string, max int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.TrimSpace(s)
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

func wrapText(s string, width, maxLines int) []string {
	// Clean up the text
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\t", " ")
	s = strings.TrimSpace(s)

	// Collapse multiple spaces
	for strings.Contains(s, "  ") {
		s = strings.ReplaceAll(s, "  ", " ")
	}

	if len(s) == 0 {
		return nil
	}

	var lines []string
	for len(s) > 0 && len(lines) < maxLines {
		if len(s) <= width {
			lines = append(lines, s)
			break
		}

		// Find a good break point
		breakAt := width
		for breakAt > width/2 && s[breakAt] != ' ' {
			breakAt--
		}
		if s[breakAt] != ' ' {
			breakAt = width // No space found, just cut
		}

		lines = append(lines, strings.TrimSpace(s[:breakAt]))
		s = strings.TrimSpace(s[breakAt:])
	}

	// Add ellipsis if truncated
	if len(s) > 0 && len(lines) == maxLines {
		lastLine := lines[maxLines-1]
		if len(lastLine) > width-3 {
			lastLine = lastLine[:width-3]
		}
		lines[maxLines-1] = lastLine + "..."
	}

	return lines
}

func openInObsidian(vaultDir, filePath string) {
	vaultName := filepath.Base(vaultDir)

	filePathWithoutExt := strings.TrimSuffix(filePath, ".md")

	url := fmt.Sprintf("obsidian://open?vault=%s&file=%s", vaultName, filePathWithoutExt)

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	}

	if cmd != nil {
		cmd.Start()
	}
}
