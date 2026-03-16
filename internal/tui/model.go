package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// teaModel is the TUI model that manages state and rendering
type teaModel struct {
	serverURL     string
	ctx           context.Context
	input         string
	output        string
	outputLines   []string
	status        string
	connected     bool
	inputMode     bool // true when typing input
	width, height int
}

// newTUIModel creates a new TUI model
func newTUIModel(serverURL string, ctx context.Context) (*teaModel, error) {
	return &teaModel{
		serverURL:   serverURL,
		ctx:         ctx,
		input:       "",
		output:      "Welcome to zen-brain TUI\nType /help for commands or start typing to chat\n",
		outputLines: []string{"Welcome to zen-brain TUI", "Type /help for commands or start typing to chat"},
		status:      "Connected",
		connected:   true,
	}, nil
}

// Init initializes the TUI model
func (m *teaModel) Init() tea.Cmd {
	// Start a tick command to update status periodically
	return tea.Batch(
		tea.Tick(time.Second*5, func(t time.Time) tea.Msg {
			return statusTickMsg(t)
		}),
	)
}

// Update handles tea messages and updates model state
func (m *teaModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Handle key presses
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			// Exit TUI
			return m, tea.Quit

		case tea.KeyEnter:
			if m.inputMode && m.input != "" {
				// Send input to server
				return m, m.sendInput()
			}

		case tea.KeyBackspace:
			if m.inputMode && len(m.input) > 0 {
				m.input = m.input[:len(m.input)-1]
			}

		default:
			// Type characters
			if m.inputMode && len(msg.Runes) > 0 {
				m.input += string(msg.Runes)
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case statusTickMsg:
		// Periodically check connection status
		return m, tea.Tick(time.Second*5, func(t time.Time) tea.Msg {
			return statusTickMsg(t)
		})

	case sendInputMsg:
		// Response from server
		m.output += msg.response
		m.outputLines = splitIntoLines(msg.response, 50) // Limit to 50 lines
		m.input = ""
		m.inputMode = false
		m.status = msg.status
		return m, nil
	}

	return m, nil
}

// View renders the TUI
func (m *teaModel) View() string {
	if m.width == 0 {
		m.width = 80
	}
	if m.height == 0 {
		m.height = 24
	}

	// Calculate layout
	headerHeight := 3
	footerHeight := 2
	outputHeight := m.height - headerHeight - footerHeight

	// Build header (simple text, no lipgloss)
	header := fmt.Sprintf(" zen-brain TUI | Server: %s | %s ", m.serverURL, m.status)

	// Build output area (scrollable)
	outputArea := ""
	if len(m.outputLines) > 0 {
		// Show last outputHeight lines
		start := 0
		if len(m.outputLines) > outputHeight {
			start = len(m.outputLines) - outputHeight
		}
		outputLines := m.outputLines[start:]
		for _, line := range outputLines {
			outputArea += truncateLine(line, m.width-4) + "\n"
		}
	}

	// Build input area
	inputArea := ""
	if m.inputMode {
		prompt := "> "
		inputArea = prompt + m.input
	} else {
		inputArea = "Type a message or /help for commands..."
	}

	// Combine with separators
	content := ""
	content += header + "\n"
	content += strings.Repeat("─", m.width) + "\n"
	content += outputArea + "\n"
	content += strings.Repeat("─", m.width) + "\n"
	content += inputArea + " "

	return content
}

// sendInput sends user input to server
func (m *teaModel) sendInput() tea.Cmd {
	inputText := m.input

	return tea.Cmd(func() tea.Msg {
		// TODO: Send to server via HTTP/WS
		// For now, echo back
		return sendInputMsg{
			response: fmt.Sprintf("Sent: %s\n[Server response placeholder]", inputText),
			status:   "Sent",
		}
	})
}

// Helper messages and functions

type statusTickMsg time.Time
type sendInputMsg struct {
	response string
	status   string
}

func splitIntoLines(text string, maxLines int) []string {
	lines := strings.Split(text, "\n")
	if len(lines) > maxLines {
		return lines[len(lines)-maxLines:]
	}
	return lines
}

func truncateLine(line string, maxLen int) string {
	if len(line) <= maxLen {
		return line
	}
	return line[:maxLen-3] + "..."
}
