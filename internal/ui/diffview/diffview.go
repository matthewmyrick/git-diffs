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
	diff     *git.FileDiff
	filePath string
	lines    []SideBySideLine
	offset   int
	width    int
	height   int
	focused  bool
	lexer    chroma.Lexer
	style    *chroma.Style
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

// visibleLines returns how many diff lines can be displayed
func (m Model) visibleLines() int {
	// height - border(2) - title(1) - column headers(2)
	visible := m.height - 5
	if visible < 1 {
		visible = 1
	}
	return visible
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
		visibleHeight := m.visibleLines()
		maxOffset := len(m.lines) - visibleHeight
		if maxOffset < 0 {
			maxOffset = 0
		}

		switch {
		case key.Matches(msg, keys.Up):
			if m.offset > 0 {
				m.offset--
			}

		case key.Matches(msg, keys.Down):
			if m.offset < maxOffset {
				m.offset++
			}

		case key.Matches(msg, keys.PageUp):
			m.offset -= visibleHeight
			if m.offset < 0 {
				m.offset = 0
			}

		case key.Matches(msg, keys.PageDown):
			m.offset += visibleHeight
			if m.offset > maxOffset {
				m.offset = maxOffset
			}

		case key.Matches(msg, keys.Home):
			m.offset = 0

		case key.Matches(msg, keys.End):
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

	visibleHeight := m.visibleLines()
	innerWidth := m.width - 4 // borders + padding

	var lines []string

	// Title
	title := "DIFF"
	if m.filePath != "" {
		title = fmt.Sprintf("DIFF: %s", filepath.Base(m.filePath))
	}
	lines = append(lines, ui.PaneTitleStyle.Render(title))

	// No diff content
	if m.diff == nil || len(m.lines) == 0 {
		lines = append(lines, ui.EmptyStateStyle.Render("Select a file to view diff"))
	} else {
		// Calculate side widths
		sideWidth := (innerWidth - 3) / 2 // -3 for separator " | "
		if sideWidth < 10 {
			sideWidth = 10
		}

		// Column headers - pad manually to match content width
		oldHeaderText := "OLD" + strings.Repeat(" ", sideWidth-3)
		newHeaderText := "NEW" + strings.Repeat(" ", sideWidth-3)
		oldHeader := lipgloss.NewStyle().Bold(true).Foreground(ui.ColorDanger).Render(oldHeaderText)
		newHeader := lipgloss.NewStyle().Bold(true).Foreground(ui.ColorSuccess).Render(newHeaderText)
		lines = append(lines, oldHeader+" | "+newHeader)
		lines = append(lines, strings.Repeat("-", sideWidth)+"-+-"+strings.Repeat("-", sideWidth))

		// Content lines
		end := m.offset + visibleHeight
		if end > len(m.lines) {
			end = len(m.lines)
		}

		lineNumWidth := 4
		for i := m.offset; i < end; i++ {
			line := m.lines[i]
			oldSide := m.renderSide(line.OldLineNum, line.OldContent, line.OldType, sideWidth, lineNumWidth)
			newSide := m.renderSide(line.NewLineNum, line.NewContent, line.NewType, sideWidth, lineNumWidth)
			lines = append(lines, oldSide+" | "+newSide)
		}

		// Scroll indicator
		if len(m.lines) > visibleHeight {
			scrollInfo := fmt.Sprintf(" [%d-%d of %d] ", m.offset+1, end, len(m.lines))
			lines = append(lines, ui.EmptyStateStyle.Render(scrollInfo))
		}
	}

	// Pad to fill height
	maxLines := m.height - 2
	for len(lines) < maxLines {
		lines = append(lines, "")
	}

	// Truncate if needed
	if len(lines) > maxLines {
		lines = lines[:maxLines]
	}

	content := strings.Join(lines, "\n")

	// Apply pane style
	var paneStyle lipgloss.Style
	if m.focused {
		paneStyle = ui.PaneFocusedStyle.Copy()
	} else {
		paneStyle = ui.PaneStyle.Copy()
	}

	return paneStyle.
		Width(m.width - 2).
		MaxHeight(m.height).
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
	lineNumRendered := ui.LineNumberStyle.Render(lineNumStr)

	// Content width
	codeWidth := width - lineNumWidth - 2
	if codeWidth < 1 {
		codeWidth = 1
	}

	// Truncate content if needed
	displayContent := content
	if len(displayContent) > codeWidth {
		displayContent = displayContent[:codeWidth-1] + "…"
	}

	// Pad content to width
	if len(displayContent) < codeWidth {
		displayContent = displayContent + strings.Repeat(" ", codeWidth-len(displayContent))
	}

	// Apply diff styling
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

	return lineNumRendered + " " + style.Render(displayContent)
}

// convertToSideBySide converts the diff hunks to side-by-side format
func (m *Model) convertToSideBySide() []SideBySideLine {
	if m.diff == nil {
		return nil
	}

	var lines []SideBySideLine

	for _, hunk := range m.diff.Hunks {
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
				// Flush pending changes
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
