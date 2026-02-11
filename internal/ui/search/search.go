package search

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/matthewmyrick/git-diffs/internal/git"
	"github.com/matthewmyrick/git-diffs/internal/ui"
	"github.com/sahilm/fuzzy"
)

// Mode represents the search mode
type Mode int

const (
	ModeFile Mode = iota
	ModeContent
)

// SelectFileMsg is sent when a file is selected
type SelectFileMsg struct {
	Path string
}

// SearchResult represents a search result
type SearchResult struct {
	Path     string
	Match    string
	Score    int
	MatchPos []int
}

// Model represents the search component
type Model struct {
	mode      Mode
	input     textinput.Model
	files     []git.ChangedFile
	results   []SearchResult
	cursor    int
	width     int
	height    int
	focused   bool
}

// New creates a new search model
func New() Model {
	ti := textinput.New()
	ti.Placeholder = "Search files..."
	ti.CharLimit = 256
	ti.Width = 40

	return Model{
		input:   ti,
		cursor:  0,
	}
}

// SetMode sets the search mode
func (m *Model) SetMode(mode Mode) {
	m.mode = mode
	m.cursor = 0
	m.results = nil
	m.input.Reset()

	switch mode {
	case ModeFile:
		m.input.Placeholder = "Search files..."
	case ModeContent:
		m.input.Placeholder = "Search content..."
	}
}

// SetFiles sets the files to search
func (m *Model) SetFiles(files []git.ChangedFile) {
	m.files = files
	m.results = nil
}

// SetSize sets the dimensions
func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.input.Width = width - 4
}

// SetFocused sets whether the search is focused
func (m *Model) SetFocused(focused bool) {
	m.focused = focused
	if focused {
		m.input.Focus()
	} else {
		m.input.Blur()
	}
}

// IsFocused returns whether the search is focused
func (m Model) IsFocused() bool {
	return m.focused
}

// Init implements tea.Model
func (m Model) Init() tea.Cmd {
	return textinput.Blink
}

// Update implements tea.Model
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	if !m.focused {
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		keys := ui.DefaultKeyMap()

		switch {
		case key.Matches(msg, keys.Escape):
			m.focused = false
			return m, nil

		case key.Matches(msg, keys.Up):
			if m.cursor > 0 {
				m.cursor--
			}
			return m, nil

		case key.Matches(msg, keys.Down):
			if m.cursor < len(m.results)-1 {
				m.cursor++
			}
			return m, nil

		case key.Matches(msg, keys.Enter):
			if len(m.results) > 0 && m.cursor < len(m.results) {
				m.focused = false
				return m, func() tea.Msg {
					return SelectFileMsg{Path: m.results[m.cursor].Path}
				}
			}
			return m, nil
		}
	}

	// Update text input
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)

	// Perform search when input changes
	m.search()

	return m, cmd
}

func (m *Model) search() {
	query := m.input.Value()
	if query == "" {
		m.results = nil
		m.cursor = 0
		return
	}

	var results []SearchResult

	switch m.mode {
	case ModeFile:
		// Fuzzy search file paths
		var paths []string
		for _, f := range m.files {
			paths = append(paths, f.Path)
		}

		matches := fuzzy.Find(query, paths)
		for _, match := range matches {
			results = append(results, SearchResult{
				Path:     match.Str,
				Match:    match.Str,
				Score:    match.Score,
				MatchPos: match.MatchedIndexes,
			})
		}

	case ModeContent:
		// Search for content in file paths (simplified - would need actual content search)
		// For now, just search file paths
		var paths []string
		for _, f := range m.files {
			paths = append(paths, f.Path)
		}

		matches := fuzzy.Find(query, paths)
		for _, match := range matches {
			results = append(results, SearchResult{
				Path:     match.Str,
				Match:    match.Str,
				Score:    match.Score,
				MatchPos: match.MatchedIndexes,
			})
		}
	}

	m.results = results
	if m.cursor >= len(m.results) {
		m.cursor = 0
	}
}

// View implements tea.Model
func (m Model) View() string {
	if !m.focused {
		return ""
	}

	var b strings.Builder

	// Title
	title := "Search Files"
	if m.mode == ModeContent {
		title = "Search Content"
	}
	b.WriteString(ui.PaneTitleStyle.Render(title))
	b.WriteString("\n\n")

	// Input
	b.WriteString(m.input.View())
	b.WriteString("\n\n")

	// Results
	if len(m.results) == 0 {
		if m.input.Value() != "" {
			b.WriteString(ui.EmptyStateStyle.Render("No matches found"))
		} else {
			b.WriteString(ui.EmptyStateStyle.Render("Type to search..."))
		}
	} else {
		maxResults := m.height - 8
		if maxResults < 1 {
			maxResults = 5
		}
		if maxResults > len(m.results) {
			maxResults = len(m.results)
		}

		for i := 0; i < maxResults; i++ {
			result := m.results[i]
			line := m.renderResult(result, i == m.cursor)
			b.WriteString(line)
			if i < maxResults-1 {
				b.WriteString("\n")
			}
		}

		if len(m.results) > maxResults {
			b.WriteString("\n")
			b.WriteString(ui.EmptyStateStyle.Render(
				strings.Repeat(" ", 2) + "..." + " and more",
			))
		}
	}

	// Footer
	b.WriteString("\n\n")
	b.WriteString(ui.FooterStyle.Render("↑↓ navigate  Enter select  Esc close"))

	// Wrap in a box
	content := b.String()

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ui.ColorPrimary).
		Background(lipgloss.Color("#1F2937")).
		Padding(1, 2).
		Width(m.width).
		Height(m.height)

	return boxStyle.Render(content)
}

func (m Model) renderResult(result SearchResult, selected bool) string {
	// Highlight matched characters
	var rendered strings.Builder
	matchSet := make(map[int]bool)
	for _, idx := range result.MatchPos {
		matchSet[idx] = true
	}

	for i, char := range result.Match {
		if matchSet[i] {
			rendered.WriteString(ui.SearchMatchStyle.Render(string(char)))
		} else {
			rendered.WriteString(string(char))
		}
	}

	line := rendered.String()

	if selected {
		return ui.SearchResultSelectedStyle.Width(m.width - 6).Render("▶ " + line)
	}
	return ui.SearchResultStyle.Width(m.width - 6).Render("  " + line)
}
