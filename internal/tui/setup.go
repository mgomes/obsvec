package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type SetupModel struct {
	apiKeyInput textinput.Model
	dirInput    textinput.Model
	focus       int
	error       string
	width       int
	height      int
}

func NewSetupModel() SetupModel {
	apiKey := textinput.New()
	apiKey.Placeholder = "Paste your Cohere API key here..."
	apiKey.Focus()
	apiKey.Width = 60
	apiKey.EchoMode = textinput.EchoPassword
	apiKey.EchoCharacter = '•'

	dirInput := textinput.New()
	dirInput.Placeholder = "/path/to/your/obsidian/vault"
	dirInput.Width = 60

	return SetupModel{
		apiKeyInput: apiKey,
		dirInput:    dirInput,
		focus:       0,
	}
}

func (m SetupModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m SetupModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit

		case "tab", "down":
			if m.focus == 0 {
				m.focus = 1
				m.apiKeyInput.Blur()
				m.dirInput.Focus()
			} else {
				m.focus = 0
				m.dirInput.Blur()
				m.apiKeyInput.Focus()
			}
			return m, nil

		case "shift+tab", "up":
			if m.focus == 1 {
				m.focus = 0
				m.dirInput.Blur()
				m.apiKeyInput.Focus()
			} else {
				m.focus = 1
				m.apiKeyInput.Blur()
				m.dirInput.Focus()
			}
			return m, nil

		case "enter":
			apiKey := strings.TrimSpace(m.apiKeyInput.Value())
			dir := strings.TrimSpace(m.dirInput.Value())

			if apiKey == "" {
				m.error = "API key is required"
				return m, nil
			}
			if dir == "" {
				m.error = "Obsidian directory is required"
				return m, nil
			}

			return m, func() tea.Msg {
				return SetupSubmitMsg{
					APIKey:      apiKey,
					ObsidianDir: dir,
				}
			}
		}

		if m.focus == 0 {
			m.apiKeyInput, cmd = m.apiKeyInput.Update(msg)
		} else {
			m.dirInput, cmd = m.dirInput.Update(msg)
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case SetupErrorMsg:
		m.error = msg.Error

	default:
		if m.focus == 0 {
			m.apiKeyInput, cmd = m.apiKeyInput.Update(msg)
		} else {
			m.dirInput, cmd = m.dirInput.Update(msg)
		}
	}

	return m, cmd
}

func (m SetupModel) View() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("obsvec - Setup") + "\n\n")
	b.WriteString("To get started, you need a Cohere API key.\n\n")
	b.WriteString("1. Go to " + activeStyle.Render("https://dashboard.cohere.com/api-keys") + "\n")
	b.WriteString("2. Create a new API key (or use an existing one)\n")
	b.WriteString("3. Copy and paste it below\n\n")

	apiKeyLabel := "Cohere API Key:"
	if m.focus == 0 {
		apiKeyLabel = activeStyle.Render("> " + apiKeyLabel)
	} else {
		apiKeyLabel = "  " + apiKeyLabel
	}
	b.WriteString(apiKeyLabel + "\n")

	style := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("63")).
		Padding(0, 1)

	b.WriteString(style.Render(m.apiKeyInput.View()) + "\n\n")

	dirLabel := "Obsidian Vault Directory:"
	if m.focus == 1 {
		dirLabel = activeStyle.Render("> " + dirLabel)
	} else {
		dirLabel = "  " + dirLabel
	}
	b.WriteString(dirLabel + "\n")
	b.WriteString(style.Render(m.dirInput.View()) + "\n")

	if m.error != "" {
		b.WriteString("\n" + errorStyle.Render("Error: "+m.error) + "\n")
	}

	b.WriteString("\n" + helpStyle.Render("tab switch field  enter submit  ctrl+c quit"))

	return b.String()
}
