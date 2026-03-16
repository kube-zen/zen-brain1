// Package tui provides a thin terminal interface for zen-brain.
// It handles only terminal rendering, input capture, and HTTP/WebSocket transport.
// All business logic (help, command parsing, status, reports, etc.) lives in the server.
package tui

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"
)

// Client is a thin TUI client for zen-brain
type Client struct {
	serverURL string
	client    *http.Client
}

// New creates a new TUI client
func New(serverURL string) *Client {
	return &Client{
		serverURL: strings.TrimSuffix(serverURL, "/"),
		client: &http.Client{
			Timeout: 5 * time.Minute, // Allow long-running tasks
		},
	}
}

// Run starts the TUI interface
func (c *Client) Run(ctx context.Context) error {
	// Check server health
	if err := c.healthCheck(); err != nil {
		return fmt.Errorf("cannot connect to server: %w", err)
	}

	// Check if terminal is interactive
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return c.runNonInteractive(ctx)
	}

	// Run interactive TUI with bubbletea
	return c.runInteractive(ctx)
}

// healthCheck verifies server connectivity
func (c *Client) healthCheck() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", c.serverURL+"/healthz", nil)
	if err != nil {
		return err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned status %d", resp.StatusCode)
	}

	return nil
}

// runInteractive runs the bubbletea TUI
func (c *Client) runInteractive(ctx context.Context) error {
	// Create initial TUI model
	model, err := newTUIModel(c.serverURL, ctx)
	if err != nil {
		return err
	}

	// Create tea program
	p := tea.NewProgram(model, tea.WithAltScreen(), tea.WithMouseCellMotion())

	// Run the program
	if _, err := p.Run(); err != nil {
		return err
	}

	return nil
}

// runNonInteractive reads from stdin and sends raw input to server
func (c *Client) runNonInteractive(ctx context.Context) error {
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := scanner.Text()
		if err := c.sendRawInput(ctx, line); err != nil {
			return err
		}
	}
	return scanner.Err()
}

// sendRawInput sends user input to the server and renders the response
func (c *Client) sendRawInput(ctx context.Context, input string) error {
	reqBody := map[string]string{
		"input": input,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.serverURL+"/api/v1/chat", bytes.NewReader(jsonBody))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	// Render response as plain text
	fmt.Println(string(body))
	return nil
}

// Styles for TUI rendering
var (
	headerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("cyan")).
			Bold(true)

	inputStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("white")).
			Background(lipgloss.Color("blue"))

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("red")).
			Bold(true)

	successStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("green")).
			Bold(true)

	infoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("yellow"))
)
