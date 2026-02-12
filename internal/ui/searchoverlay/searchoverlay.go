package searchoverlay

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/matthewmyrick/git-diffs/internal/ui"
	"github.com/sahilm/fuzzy"
)

// SearchLine represents a searchable line from the diff
type SearchLine struct {
	LineNum int
	Content string
	Type    string // "add", "del", "context", "header"
	OrigIdx int    // Original index in the diff lines
}

// CloseMsg is sent when the search overlay should close
type CloseMsg struct{}

// JumpToLineMsg is sent when user selects a line to jump to
type JumpToLineMsg struct {
	OrigIdx int
}

// Model represents the search overlay component
type Model struct {
	lines       []SearchLine
	matches     []fuzzy.Match
	searchInput textinput.Model
	cursor      int
	offset      int
	width       int
	height      int
	active      bool
	viewMode    string // "both", "new", "old"
}

// New creates a new search overlay model
func New() Model {
	ti := textinput.New()
	ti.Placeholder = "Search content..."
	ti.CharLimit = 200
	ti.Width = 40

	return Model{
		searchInput: ti,
		viewMode:    "both",
	}
}

// SetLines sets the searchable lines
func (m *Model) SetLines(lines []SearchLine) {
	m.lines = lines
	m.matches = nil
	m.cursor = 0
	m.offset = 0
}

// SetSize sets the overlay dimensions
func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// SetViewMode sets the current view mode
func (m *Model) SetViewMode(mode string) {
	m.viewMode = mode
}

// Open activates the search overlay
func (m *Model) Open() {
	m.active = true
	m.searchInput.SetValue("")
	m.searchInput.Focus()
	m.cursor = 0
	m.offset = 0
	m.updateMatches()
}

// Close deactivates the search overlay
func (m *Model) Close() {
	m.active = false
	m.searchInput.Blur()
}

// IsActive returns whether the overlay is active
func (m Model) IsActive() bool {
	return m.active
}

// Init implements tea.Model
func (m Model) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	if !m.active {
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			m.Close()
			return m, func() tea.Msg { return CloseMsg{} }

		case "enter":
			if len(m.matches) > 0 && m.cursor < len(m.matches) {
				idx := m.matches[m.cursor].Index
				origIdx := m.lines[idx].OrigIdx
				m.Close()
				return m, func() tea.Msg { return JumpToLineMsg{OrigIdx: origIdx} }
			}
			return m, nil

		case "up", "ctrl+k":
			if m.cursor > 0 {
				m.cursor--
				m.ensureVisible()
			}
			return m, nil

		case "down", "ctrl+j":
			if m.cursor < len(m.matches)-1 {
				m.cursor++
				m.ensureVisible()
			}
			return m, nil

		case "ctrl+u":
			m.cursor -= 10
			if m.cursor < 0 {
				m.cursor = 0
			}
			m.ensureVisible()
			return m, nil

		case "ctrl+d":
			m.cursor += 10
			if m.cursor >= len(m.matches) {
				m.cursor = len(m.matches) - 1
			}
			if m.cursor < 0 {
				m.cursor = 0
			}
			m.ensureVisible()
			return m, nil

		default:
			var cmd tea.Cmd
			m.searchInput, cmd = m.searchInput.Update(msg)
			m.updateMatches()
			m.cursor = 0
			m.offset = 0
			return m, cmd
		}
	}

	return m, nil
}

func (m *Model) updateMatches() {
	query := strings.ReplaceAll(m.searchInput.Value(), " ", "")
	if query == "" {
		// Show all lines when no query
		m.matches = make([]fuzzy.Match, len(m.lines))
		for i := range m.lines {
			m.matches[i] = fuzzy.Match{Index: i}
		}
		return
	}

	// Build searchable strings
	var strs []string
	for _, line := range m.lines {
		strs = append(strs, line.Content)
	}

	m.matches = fuzzy.Find(query, strs)
}

func (m *Model) ensureVisible() {
	visibleHeight := m.contentHeight()
	if m.cursor < m.offset {
		m.offset = m.cursor
	} else if m.cursor >= m.offset+visibleHeight {
		m.offset = m.cursor - visibleHeight + 1
	}
}

func (m Model) contentHeight() int {
	// Fixed content height based on overlay size
	h := m.overlayHeight() - 6 // borders, title, search bar, separator, padding
	if h < 3 {
		h = 3
	}
	return h
}

func (m Model) overlayHeight() int {
	h := int(float64(m.height) * 0.80)
	if h < 10 {
		h = 10
	}
	return h
}

func (m Model) overlayWidth() int {
	w := int(float64(m.width) * 0.85)
	if w < 60 {
		w = 60
	}
	return w
}

// RenderOverlay renders the overlay on top of a background
func (m Model) RenderOverlay(background string) string {
	if !m.active || m.width == 0 || m.height == 0 {
		return background
	}

	overlayWidth := m.overlayWidth()
	overlayHeight := m.overlayHeight()
	contentHeight := m.contentHeight()

	// Left pane (results) takes 40%, right pane (preview) takes 60%
	leftWidth := (overlayWidth - 6) * 40 / 100 // -6 for borders, padding, divider
	rightWidth := (overlayWidth - 6) - leftWidth - 3

	// Build left pane content
	var leftLines []string

	// Search input
	searchLine := m.renderSearchInput(leftWidth)
	leftLines = append(leftLines, searchLine)
	leftLines = append(leftLines, lipgloss.NewStyle().Foreground(ui.ColorMuted).Render(strings.Repeat("─", leftWidth)))

	// Results
	if len(m.matches) == 0 {
		leftLines = append(leftLines, ui.EmptyStateStyle.Render("No matches"))
	} else {
		end := m.offset + contentHeight
		if end > len(m.matches) {
			end = len(m.matches)
		}

		for i := m.offset; i < end; i++ {
			match := m.matches[i]
			line := m.lines[match.Index]
			leftLines = append(leftLines, m.renderResultLine(line, i == m.cursor, leftWidth, match))
		}
	}

	// Pad to fixed height
	for len(leftLines) < contentHeight+2 {
		leftLines = append(leftLines, strings.Repeat(" ", leftWidth))
	}
	// Truncate to fixed height
	if len(leftLines) > contentHeight+2 {
		leftLines = leftLines[:contentHeight+2]
	}

	// Build right pane content
	var rightLines []string
	rightLines = append(rightLines, lipgloss.NewStyle().Bold(true).Foreground(ui.ColorPrimary).Render("Preview"))
	rightLines = append(rightLines, lipgloss.NewStyle().Foreground(ui.ColorMuted).Render(strings.Repeat("─", rightWidth)))

	if len(m.matches) > 0 && m.cursor < len(m.matches) {
		match := m.matches[m.cursor]
		lineIdx := match.Index
		rightLines = append(rightLines, m.renderPreview(lineIdx, rightWidth, contentHeight)...)
	} else {
		rightLines = append(rightLines, ui.EmptyStateStyle.Render("Select a result"))
	}

	// Pad to fixed height
	for len(rightLines) < contentHeight+2 {
		rightLines = append(rightLines, strings.Repeat(" ", rightWidth))
	}
	// Truncate to fixed height
	if len(rightLines) > contentHeight+2 {
		rightLines = rightLines[:contentHeight+2]
	}

	// Render panes with fixed dimensions
	leftPane := lipgloss.NewStyle().
		Width(leftWidth).
		Height(contentHeight + 2).
		Render(strings.Join(leftLines, "\n"))

	rightPane := lipgloss.NewStyle().
		Width(rightWidth).
		Height(contentHeight + 2).
		Render(strings.Join(rightLines, "\n"))

	// Combine with divider
	dividerStyle := lipgloss.NewStyle().Foreground(ui.ColorMuted)
	divider := dividerStyle.Render(" │ ")
	innerContent := lipgloss.JoinHorizontal(lipgloss.Top, leftPane, divider, rightPane)

	// Create the overlay box
	overlayBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ui.ColorPrimary).
		Padding(0, 1).
		Width(overlayWidth).
		Height(overlayHeight).
		Render(innerContent)

	// Composite overlay on top of dimmed background
	return m.compositeOverlay(background, overlayBox, overlayWidth, overlayHeight)
}

// compositeOverlay places the overlay on top of a dimmed background
func (m Model) compositeOverlay(background, overlay string, overlayW, overlayH int) string {
	bgLines := strings.Split(background, "\n")

	// Ensure background has enough lines
	for len(bgLines) < m.height {
		bgLines = append(bgLines, "")
	}

	// Dim the background
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#444444"))
	for i := range bgLines {
		// Strip ANSI and dim
		plain := stripAnsi(bgLines[i])
		// Pad to full width
		if len(plain) < m.width {
			plain = plain + strings.Repeat(" ", m.width-len(plain))
		}
		bgLines[i] = dimStyle.Render(plain)
	}

	// Calculate overlay position (centered)
	startRow := (m.height - overlayH) / 2
	startCol := (m.width - overlayW) / 2

	// Split overlay into lines
	overlayLines := strings.Split(overlay, "\n")

	// Composite overlay onto background
	for i, overlayLine := range overlayLines {
		bgRow := startRow + i
		if bgRow >= 0 && bgRow < len(bgLines) {
			bgLines[bgRow] = m.insertOverlayLine(bgLines[bgRow], overlayLine, startCol)
		}
	}

	return strings.Join(bgLines[:m.height], "\n")
}

// insertOverlayLine inserts overlay content at a specific column position
func (m Model) insertOverlayLine(bgLine, overlayLine string, startCol int) string {
	// For simplicity, just replace the middle portion
	// This is a basic implementation - we're replacing characters
	bgRunes := []rune(stripAnsi(bgLine))

	// Ensure bg is wide enough
	for len(bgRunes) < m.width {
		bgRunes = append(bgRunes, ' ')
	}

	// Create the result: dimmed left + overlay + dimmed right
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#444444"))

	left := ""
	if startCol > 0 {
		left = dimStyle.Render(string(bgRunes[:startCol]))
	}

	right := ""
	overlayWidth := lipgloss.Width(overlayLine)
	endCol := startCol + overlayWidth
	if endCol < len(bgRunes) {
		right = dimStyle.Render(string(bgRunes[endCol:]))
	}

	return left + overlayLine + right
}

// stripAnsi removes ANSI escape codes from a string
func stripAnsi(s string) string {
	var result strings.Builder
	inEscape := false
	for _, r := range s {
		if r == '\x1b' {
			inEscape = true
			continue
		}
		if inEscape {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				inEscape = false
			}
			continue
		}
		result.WriteRune(r)
	}
	return result.String()
}

func (m Model) renderSearchInput(width int) string {
	prefix := lipgloss.NewStyle().Foreground(ui.ColorPrimary).Bold(true).Render("> ")

	// Constrain input width
	inputWidth := width - 15
	if inputWidth < 10 {
		inputWidth = 10
	}
	m.searchInput.Width = inputWidth
	input := m.searchInput.View()

	count := fmt.Sprintf(" [%d]", len(m.matches))
	countStyled := lipgloss.NewStyle().Foreground(ui.ColorMuted).Render(count)

	return prefix + input + countStyled
}

func (m Model) renderResultLine(line SearchLine, selected bool, width int, match fuzzy.Match) string {
	// Type indicator
	var typeIndicator string
	var typeColor lipgloss.Color
	switch line.Type {
	case "add":
		typeIndicator = "+"
		typeColor = ui.ColorSuccess
	case "del":
		typeIndicator = "-"
		typeColor = ui.ColorDanger
	default:
		typeIndicator = " "
		typeColor = ui.ColorMuted
	}

	indicator := lipgloss.NewStyle().Foreground(typeColor).Render(typeIndicator)

	// Line number
	lineNum := fmt.Sprintf("%4d", line.LineNum)
	if line.LineNum == 0 {
		lineNum = "    "
	}
	lineNumStyled := lipgloss.NewStyle().Foreground(ui.ColorMuted).Render(lineNum)

	// Content (truncate if needed)
	content := line.Content
	maxContentWidth := width - 10
	if maxContentWidth < 5 {
		maxContentWidth = 5
	}
	if len(content) > maxContentWidth {
		content = content[:maxContentWidth-1] + "…"
	}

	// Highlight matched characters
	var styledContent string
	if len(match.MatchedIndexes) > 0 && m.searchInput.Value() != "" {
		styledContent = m.highlightMatches(content, match.MatchedIndexes)
	} else {
		styledContent = content
	}

	// Cursor indicator
	cursor := "  "
	if selected {
		cursor = lipgloss.NewStyle().Foreground(ui.ColorPrimary).Bold(true).Render("> ")
	}

	lineStr := fmt.Sprintf("%s%s %s %s", cursor, indicator, lineNumStyled, styledContent)

	// Pad to width
	lineWidth := lipgloss.Width(lineStr)
	if lineWidth < width {
		lineStr += strings.Repeat(" ", width-lineWidth)
	}

	if selected {
		return lipgloss.NewStyle().
			Background(lipgloss.Color("#2a2a3a")).
			Render(lineStr)
	}

	return lineStr
}

func (m Model) highlightMatches(content string, matchedIndexes []int) string {
	if len(matchedIndexes) == 0 {
		return content
	}

	matchSet := make(map[int]bool)
	for _, idx := range matchedIndexes {
		if idx < len(content) {
			matchSet[idx] = true
		}
	}

	var result strings.Builder
	highlightStyle := lipgloss.NewStyle().Foreground(ui.ColorWarning).Bold(true)

	for i, char := range content {
		if matchSet[i] {
			result.WriteString(highlightStyle.Render(string(char)))
		} else {
			result.WriteRune(char)
		}
	}

	return result.String()
}

func (m Model) renderPreview(centerIdx int, width int, height int) []string {
	var lines []string

	// Show context around the selected line
	contextSize := height / 2
	startIdx := centerIdx - contextSize
	if startIdx < 0 {
		startIdx = 0
	}
	endIdx := startIdx + height
	if endIdx > len(m.lines) {
		endIdx = len(m.lines)
		startIdx = endIdx - height
		if startIdx < 0 {
			startIdx = 0
		}
	}

	for i := startIdx; i < endIdx; i++ {
		line := m.lines[i]
		isCenter := i == centerIdx

		// Styling based on line type
		var bgColor lipgloss.Color
		var fgColor lipgloss.Color
		var prefix string

		switch line.Type {
		case "add":
			bgColor = lipgloss.Color("#0a1a0a")
			fgColor = lipgloss.Color("#88cc88")
			prefix = "+"
		case "del":
			bgColor = lipgloss.Color("#1a0a0a")
			fgColor = lipgloss.Color("#cc8888")
			prefix = "-"
		case "header":
			bgColor = lipgloss.Color("#0a0a1a")
			fgColor = lipgloss.Color("#8888cc")
			prefix = "@"
		default:
			bgColor = lipgloss.Color("")
			fgColor = ui.ColorTextMuted
			prefix = " "
		}

		// Line number
		lineNum := fmt.Sprintf("%4d", line.LineNum)
		if line.LineNum == 0 {
			lineNum = "    "
		}

		// Content
		content := line.Content
		maxWidth := width - 8
		if maxWidth < 5 {
			maxWidth = 5
		}
		if len(content) > maxWidth {
			content = content[:maxWidth-1] + "…"
		}

		// Pad content
		if len(content) < maxWidth {
			content = content + strings.Repeat(" ", maxWidth-len(content))
		}

		// Build line
		lineNumStyle := lipgloss.NewStyle().Foreground(ui.ColorMuted)
		contentStyle := lipgloss.NewStyle().Background(bgColor).Foreground(fgColor)

		renderedLine := prefix + " " + lineNumStyle.Render(lineNum) + " " + contentStyle.Render(content)

		// Highlight center line
		if isCenter {
			renderedLine = lipgloss.NewStyle().
				Background(lipgloss.Color("#3a3a5a")).
				Bold(true).
				Render(renderedLine)
		}

		lines = append(lines, renderedLine)
	}

	return lines
}

// View returns empty - use RenderOverlay instead
func (m Model) View() string {
	return ""
}
