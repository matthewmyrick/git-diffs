package diffview

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/matthewmyrick/git-diffs/internal/git"
	"github.com/matthewmyrick/git-diffs/internal/ui"
)

// SideBySideLine represents a line in the side-by-side view
type SideBySideLine struct {
	OldLineNum int
	OldContent string
	OldType    git.DiffLineType
	NewLineNum int
	NewContent string
	NewType    git.DiffLineType
}

// Model represents the diff view component
type Model struct {
	diff       *git.FileDiff
	filePath   string
	lines      []SideBySideLine
	offset     int
	width      int
	height     int
	focused    bool
	lexer      chroma.Lexer
	style      *chroma.Style
}

// New creates a new diff view model
func New() Model {
	return Model{
		style: styles.Get("monokai"),
	}
}

// SetDiff sets the diff to display
func (m *Model) SetDiff(diff *git.FileDiff, filePath string) {
	m.diff = diff
	m.filePath = filePath
	m.offset = 0

	// Set up lexer based on file extension
	m.lexer = lexers.Match(filePath)
	if m.lexer == nil {
		m.lexer = lexers.Fallback
	}
	m.lexer = chroma.Coalesce(m.lexer)

	// Convert diff to side-by-side format
	m.lines = m.convertToSideBySide()
}

// SetSize sets the dimensions
func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// SetFocused sets whether this component is focused
func (m *Model) SetFocused(focused bool) {
	m.focused = focused
}

// IsFocused returns whether this component is focused
func (m Model) IsFocused() bool {
	return m.focused
}

// Init implements tea.Model
func (m Model) Init() tea.Cmd {
	return nil
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
		case key.Matches(msg, keys.Up):
			if m.offset > 0 {
				m.offset--
			}

		case key.Matches(msg, keys.Down):
			maxOffset := len(m.lines) - (m.height - 4)
			if maxOffset < 0 {
				maxOffset = 0
			}
			if m.offset < maxOffset {
				m.offset++
			}

		case key.Matches(msg, keys.PageUp):
			pageSize := m.height - 4
			m.offset -= pageSize
			if m.offset < 0 {
				m.offset = 0
			}

		case key.Matches(msg, keys.PageDown):
			pageSize := m.height - 4
			m.offset += pageSize
			maxOffset := len(m.lines) - (m.height - 4)
			if maxOffset < 0 {
				maxOffset = 0
			}
			if m.offset > maxOffset {
				m.offset = maxOffset
			}

		case key.Matches(msg, keys.Home):
			m.offset = 0

		case key.Matches(msg, keys.End):
			maxOffset := len(m.lines) - (m.height - 4)
			if maxOffset < 0 {
				maxOffset = 0
			}
			m.offset = maxOffset
		}
	}

	return m, nil
}

// View implements tea.Model
func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}

	// Calculate dimensions
	contentHeight := m.height - 2 // borders
	contentWidth := m.width - 2   // borders

	// Title
	title := "DIFF"
	if m.filePath != "" {
		title = fmt.Sprintf("DIFF: %s", filepath.Base(m.filePath))
	}
	titleRendered := ui.PaneTitleStyle.Render(title)

	// No diff content
	if m.diff == nil || len(m.lines) == 0 {
		emptyMsg := ui.EmptyStateStyle.Render("Select a file to view diff")
		content := titleRendered + "\n" + emptyMsg

		// Pad to fill height
		lines := 2
		for i := lines; i < contentHeight; i++ {
			content += "\n"
		}

		var paneStyle lipgloss.Style
		if m.focused {
			paneStyle = ui.PaneFocusedStyle
		} else {
			paneStyle = ui.PaneStyle
		}

		return paneStyle.
			Width(contentWidth).
			Height(contentHeight).
			Render(content)
	}

	// Calculate side widths
	sideWidth := (contentWidth - 3) / 2 // -3 for separator
	lineNumWidth := 4

	// Build the diff view
	var b strings.Builder

	// Column headers
	oldHeader := lipgloss.NewStyle().
		Width(sideWidth).
		Bold(true).
		Foreground(ui.ColorDanger).
		Render("OLD")
	newHeader := lipgloss.NewStyle().
		Width(sideWidth).
		Bold(true).
		Foreground(ui.ColorSuccess).
		Render("NEW")
	b.WriteString(oldHeader + " │ " + newHeader + "\n")
	b.WriteString(strings.Repeat("─", sideWidth) + "─┼─" + strings.Repeat("─", sideWidth) + "\n")

	// Content lines
	visibleHeight := contentHeight - 4 // title + headers + separator
	if visibleHeight < 1 {
		visibleHeight = 1
	}

	end := m.offset + visibleHeight
	if end > len(m.lines) {
		end = len(m.lines)
	}

	for i := m.offset; i < end; i++ {
		line := m.lines[i]
		oldSide := m.renderSide(line.OldLineNum, line.OldContent, line.OldType, sideWidth, lineNumWidth)
		newSide := m.renderSide(line.NewLineNum, line.NewContent, line.NewType, sideWidth, lineNumWidth)
		b.WriteString(oldSide + " │ " + newSide)
		if i < end-1 {
			b.WriteString("\n")
		}
	}

	// Pad remaining lines
	renderedLines := end - m.offset
	for i := renderedLines; i < visibleHeight; i++ {
		emptyLine := strings.Repeat(" ", sideWidth) + " │ " + strings.Repeat(" ", sideWidth)
		b.WriteString("\n" + emptyLine)
	}

	content := titleRendered + "\n" + b.String()

	// Apply pane style
	var paneStyle lipgloss.Style
	if m.focused {
		paneStyle = ui.PaneFocusedStyle
	} else {
		paneStyle = ui.PaneStyle
	}

	return paneStyle.
		Width(contentWidth).
		Height(contentHeight).
		Render(content)
}

func (m Model) renderSide(lineNum int, content string, lineType git.DiffLineType, width, lineNumWidth int) string {
	// Line number
	var lineNumStr string
	if lineNum > 0 {
		lineNumStr = fmt.Sprintf("%*d", lineNumWidth, lineNum)
	} else {
		lineNumStr = strings.Repeat(" ", lineNumWidth)
	}
	lineNumRendered := ui.LineNumberStyle.Width(lineNumWidth).Render(lineNumStr)

	// Content width
	codeWidth := width - lineNumWidth - 1
	if codeWidth < 1 {
		codeWidth = 1
	}

	// Truncate or pad content
	displayContent := content
	if len(displayContent) > codeWidth {
		displayContent = displayContent[:codeWidth-1] + "…"
	}

	// Apply syntax highlighting and diff styling
	var style lipgloss.Style
	switch lineType {
	case git.DiffLineAddition:
		style = ui.DiffAdditionStyle
	case git.DiffLineDeletion:
		style = ui.DiffDeletionStyle
	case git.DiffLineHeader:
		style = ui.DiffHeaderStyle
	default:
		style = ui.DiffContextStyle
	}

	// Highlight code with chroma if possible
	highlighted := m.highlightCode(displayContent)
	if highlighted != "" {
		displayContent = highlighted
	}

	// Pad to fill width
	displayContent = style.Width(codeWidth).Render(displayContent)

	return lineNumRendered + " " + displayContent
}

func (m Model) highlightCode(code string) string {
	if m.lexer == nil || m.style == nil {
		return code
	}

	iterator, err := m.lexer.Tokenise(nil, code)
	if err != nil {
		return code
	}

	var result strings.Builder
	for token := iterator(); token != chroma.EOF; token = iterator() {
		entry := m.style.Get(token.Type)
		style := lipgloss.NewStyle()

		if entry.Colour.IsSet() {
			style = style.Foreground(lipgloss.Color(entry.Colour.String()))
		}
		if entry.Bold == chroma.Yes {
			style = style.Bold(true)
		}
		if entry.Italic == chroma.Yes {
			style = style.Italic(true)
		}

		result.WriteString(style.Render(token.Value))
	}

	return result.String()
}

// convertToSideBySide converts the diff hunks to side-by-side format
func (m *Model) convertToSideBySide() []SideBySideLine {
	if m.diff == nil {
		return nil
	}

	var lines []SideBySideLine

	for _, hunk := range m.diff.Hunks {
		// Collect additions and deletions for alignment
		var deletions []git.DiffLine
		var additions []git.DiffLine

		for _, line := range hunk.Lines {
			switch line.Type {
			case git.DiffLineHeader:
				lines = append(lines, SideBySideLine{
					OldContent: line.Content,
					OldType:    git.DiffLineHeader,
					NewContent: line.Content,
					NewType:    git.DiffLineHeader,
				})

			case git.DiffLineContext:
				// Flush any pending changes
				lines = append(lines, m.alignChanges(deletions, additions)...)
				deletions = nil
				additions = nil

				lines = append(lines, SideBySideLine{
					OldLineNum: line.OldLineNum,
					OldContent: line.Content,
					OldType:    git.DiffLineContext,
					NewLineNum: line.NewLineNum,
					NewContent: line.Content,
					NewType:    git.DiffLineContext,
				})

			case git.DiffLineDeletion:
				deletions = append(deletions, line)

			case git.DiffLineAddition:
				additions = append(additions, line)
			}
		}

		// Flush remaining changes
		lines = append(lines, m.alignChanges(deletions, additions)...)
	}

	return lines
}

// alignChanges aligns deletions and additions side by side
func (m *Model) alignChanges(deletions, additions []git.DiffLine) []SideBySideLine {
	var result []SideBySideLine

	maxLen := len(deletions)
	if len(additions) > maxLen {
		maxLen = len(additions)
	}

	for i := 0; i < maxLen; i++ {
		line := SideBySideLine{}

		if i < len(deletions) {
			line.OldLineNum = deletions[i].OldLineNum
			line.OldContent = deletions[i].Content
			line.OldType = git.DiffLineDeletion
		}

		if i < len(additions) {
			line.NewLineNum = additions[i].NewLineNum
			line.NewContent = additions[i].Content
			line.NewType = git.DiffLineAddition
		}

		result = append(result, line)
	}

	return result
}

// FilePath returns the current file path
func (m Model) FilePath() string {
	return m.filePath
}

// Clear clears the diff view
func (m *Model) Clear() {
	m.diff = nil
	m.filePath = ""
	m.lines = nil
	m.offset = 0
}
