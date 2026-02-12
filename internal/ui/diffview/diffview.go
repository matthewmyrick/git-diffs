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

// ViewMode represents the diff view mode
type ViewMode int

const (
	ViewBoth ViewMode = iota // Side-by-side (default)
	ViewNew                  // Only new/added content
	ViewOld                  // Only old/deleted content
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
	cursor   int
	width    int
	height   int
	focused  bool
	lexer    chroma.Lexer
	style    *chroma.Style
	viewMode ViewMode
}

// New creates a new diff view model
func New() Model {
	return Model{
		style:    styles.Get("monokai"),
		viewMode: ViewBoth,
		cursor:   0,
	}
}

// SetDiff sets the diff to display
func (m *Model) SetDiff(diff *git.FileDiff, filePath string) {
	m.diff = diff
	m.filePath = filePath
	m.offset = 0
	m.cursor = 0

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
	// height - border(2) - title(1) - tabs(1) - column headers(2)
	visible := m.height - 6
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
		maxCursor := len(m.lines) - 1
		if maxCursor < 0 {
			maxCursor = 0
		}

		switch {
		case key.Matches(msg, keys.BracketLeft):
			// Previous view mode
			if m.viewMode > 0 {
				m.viewMode--
			} else {
				m.viewMode = ViewOld
			}
			m.offset = 0
			m.cursor = 0

		case key.Matches(msg, keys.BracketRight):
			// Next view mode
			if m.viewMode < ViewOld {
				m.viewMode++
			} else {
				m.viewMode = ViewBoth
			}
			m.offset = 0
			m.cursor = 0

		case key.Matches(msg, keys.Up):
			if m.cursor > 0 {
				m.cursor--
				// Scroll up if cursor goes above visible area
				if m.cursor < m.offset {
					m.offset = m.cursor
				}
			}

		case key.Matches(msg, keys.Down):
			if m.cursor < maxCursor {
				m.cursor++
				// Scroll down if cursor goes below visible area
				if m.cursor >= m.offset+visibleHeight {
					m.offset = m.cursor - visibleHeight + 1
				}
			}

		case key.Matches(msg, keys.PageUp):
			m.cursor -= visibleHeight
			if m.cursor < 0 {
				m.cursor = 0
			}
			m.offset = m.cursor

		case key.Matches(msg, keys.PageDown):
			m.cursor += visibleHeight
			if m.cursor > maxCursor {
				m.cursor = maxCursor
			}
			if m.cursor >= m.offset+visibleHeight {
				m.offset = m.cursor - visibleHeight + 1
			}

		case key.Matches(msg, keys.Home):
			m.cursor = 0
			m.offset = 0

		case key.Matches(msg, keys.End):
			m.cursor = maxCursor
			if m.cursor >= visibleHeight {
				m.offset = m.cursor - visibleHeight + 1
			}
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

	// Tabs
	lines = append(lines, m.renderTabs())

	// No diff content
	if m.diff == nil || len(m.lines) == 0 {
		lines = append(lines, ui.EmptyStateStyle.Render("Select a file to view diff"))
	} else {
		switch m.viewMode {
		case ViewBoth:
			lines = append(lines, m.renderBothView(innerWidth, visibleHeight)...)
		case ViewNew:
			lines = append(lines, m.renderSingleView(innerWidth, visibleHeight, true)...)
		case ViewOld:
			lines = append(lines, m.renderSingleView(innerWidth, visibleHeight, false)...)
		}
	}

	// Pad to fill height
	maxLines := m.height - 2
	for len(lines) < maxLines {
		lines = append(lines, "")
	}
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

func (m Model) renderTabs() string {
	modes := []string{"Both", "New", "Old"}
	var tabs []string

	for i, mode := range modes {
		style := lipgloss.NewStyle().Padding(0, 1)
		if ViewMode(i) == m.viewMode {
			style = style.Bold(true).Foreground(ui.ColorPrimary)
			tabs = append(tabs, style.Render("["+mode+"]"))
		} else {
			style = style.Foreground(ui.ColorMuted)
			tabs = append(tabs, style.Render(mode))
		}
	}

	return strings.Join(tabs, " ")
}

func (m Model) renderBothView(innerWidth, visibleHeight int) []string {
	var lines []string

	// Calculate side widths (account for cursor indicator)
	sideWidth := (innerWidth - 5) / 2 // -5 for separator " | " and cursor "> "
	if sideWidth < 10 {
		sideWidth = 10
	}

	// Column headers
	actualSideWidth := sideWidth - 1
	oldHeaderText := "OLD" + strings.Repeat(" ", actualSideWidth-3)
	newHeaderText := "NEW" + strings.Repeat(" ", actualSideWidth-3)
	oldHeader := lipgloss.NewStyle().Bold(true).Foreground(ui.ColorDanger).Render(oldHeaderText)
	newHeader := lipgloss.NewStyle().Bold(true).Foreground(ui.ColorSuccess).Render(newHeaderText)
	lines = append(lines, "  "+oldHeader+" | "+newHeader)
	lines = append(lines, "  "+strings.Repeat("-", actualSideWidth)+"-+-"+strings.Repeat("-", actualSideWidth))

	// Content lines
	end := m.offset + visibleHeight
	if end > len(m.lines) {
		end = len(m.lines)
	}

	lineNumWidth := 4
	for i := m.offset; i < end; i++ {
		line := m.lines[i]
		isCursor := i == m.cursor && m.focused

		cursor := "  "
		if isCursor {
			cursor = "> "
		}
		oldSide := m.renderSide(line.OldLineNum, line.OldContent, line.OldType, sideWidth, lineNumWidth, isCursor)
		newSide := m.renderSide(line.NewLineNum, line.NewContent, line.NewType, sideWidth, lineNumWidth, isCursor)
		lines = append(lines, cursor+oldSide+" | "+newSide)
	}

	// Scroll indicator
	if len(m.lines) > visibleHeight {
		scrollInfo := fmt.Sprintf(" [%d-%d of %d] (line %d)", m.offset+1, end, len(m.lines), m.cursor+1)
		lines = append(lines, "  "+ui.EmptyStateStyle.Render(scrollInfo))
	}

	return lines
}

func (m Model) renderSingleView(innerWidth, visibleHeight int, showNew bool) []string {
	var lines []string

	// Full width for single view (account for cursor)
	fullWidth := innerWidth - 2
	if fullWidth < 20 {
		fullWidth = 20
	}

	// Column header
	var headerText string
	var headerColor lipgloss.Color
	if showNew {
		headerText = "NEW"
		headerColor = ui.ColorSuccess
	} else {
		headerText = "OLD"
		headerColor = ui.ColorDanger
	}
	header := lipgloss.NewStyle().Bold(true).Foreground(headerColor).Render(headerText)
	lines = append(lines, "  "+header)
	lines = append(lines, "  "+strings.Repeat("-", fullWidth-2))

	// Filter and display lines
	lineNumWidth := 5
	contentWidth := fullWidth - lineNumWidth - 2

	displayedCount := 0
	skipped := 0

	for origIdx, line := range m.lines {
		var lineNum int
		var content string
		var lineType git.DiffLineType

		if showNew {
			// Show additions and context
			if line.NewType == git.DiffLineAddition || line.NewType == git.DiffLineContext || line.NewType == git.DiffLineHeader {
				lineNum = line.NewLineNum
				content = line.NewContent
				lineType = line.NewType
			} else if line.OldType == git.DiffLineContext || line.OldType == git.DiffLineHeader {
				lineNum = line.NewLineNum
				content = line.OldContent
				lineType = line.OldType
			} else {
				continue // Skip deletions in new view
			}
		} else {
			// Show deletions and context
			if line.OldType == git.DiffLineDeletion || line.OldType == git.DiffLineContext || line.OldType == git.DiffLineHeader {
				lineNum = line.OldLineNum
				content = line.OldContent
				lineType = line.OldType
			} else if line.NewType == git.DiffLineContext || line.NewType == git.DiffLineHeader {
				lineNum = line.OldLineNum
				content = line.NewContent
				lineType = line.NewType
			} else {
				continue // Skip additions in old view
			}
		}

		// Handle offset
		if skipped < m.offset {
			skipped++
			continue
		}

		if displayedCount >= visibleHeight {
			break
		}

		isCursor := origIdx == m.cursor && m.focused
		cursor := "  "
		if isCursor {
			cursor = "> "
		}

		renderedLine := m.renderFullWidthLine(lineNum, content, lineType, contentWidth, lineNumWidth, isCursor)
		lines = append(lines, cursor+renderedLine)
		displayedCount++
	}

	return lines
}

func (m Model) renderFullWidthLine(lineNum int, content string, lineType git.DiffLineType, contentWidth, lineNumWidth int, isCursor bool) string {
	// Line number
	var lineNumStr string
	if lineNum > 0 {
		lineNumStr = fmt.Sprintf("%*d", lineNumWidth, lineNum)
	} else {
		lineNumStr = strings.Repeat(" ", lineNumWidth)
	}
	lineNumRendered := ui.LineNumberStyle.Render(lineNumStr)

	// Truncate content if needed
	displayContent := content
	if len(displayContent) > contentWidth {
		displayContent = displayContent[:contentWidth-1] + "…"
	}

	// Determine background color based on diff type (subtle tints)
	var bgColor lipgloss.Color
	var defaultFg lipgloss.Color
	switch lineType {
	case git.DiffLineAddition:
		bgColor = lipgloss.Color("#0a1a0a")  // Very subtle dark green
		defaultFg = lipgloss.Color("#88cc88")
	case git.DiffLineDeletion:
		bgColor = lipgloss.Color("#1a0a0a")  // Very subtle dark red
		defaultFg = lipgloss.Color("#cc8888")
	case git.DiffLineHeader:
		bgColor = lipgloss.Color("#0a0a1a")  // Very subtle dark blue
		defaultFg = lipgloss.Color("#8888cc")
	default:
		bgColor = lipgloss.Color("")
		defaultFg = ui.ColorTextMuted
	}

	// Apply syntax highlighting
	var result strings.Builder
	currentLen := 0

	if m.lexer != nil && m.style != nil && lineType != git.DiffLineHeader {
		iterator, err := m.lexer.Tokenise(nil, displayContent)
		if err == nil {
			for token := iterator(); token != chroma.EOF; token = iterator() {
				tokenText := token.Value

				if currentLen+len(tokenText) > contentWidth {
					tokenText = tokenText[:contentWidth-currentLen]
				}
				if len(tokenText) == 0 {
					break
				}

				entry := m.style.Get(token.Type)
				style := lipgloss.NewStyle().Background(bgColor)

				if entry.Colour.IsSet() {
					style = style.Foreground(lipgloss.Color(entry.Colour.String()))
				} else {
					style = style.Foreground(defaultFg)
				}
				if entry.Bold == chroma.Yes {
					style = style.Bold(true)
				}

				result.WriteString(style.Render(tokenText))
				currentLen += len(tokenText)

				if currentLen >= contentWidth {
					break
				}
			}
		}
	}

	if currentLen == 0 {
		style := lipgloss.NewStyle().Background(bgColor).Foreground(defaultFg)
		result.WriteString(style.Render(displayContent))
		currentLen = len(displayContent)
	}

	if currentLen < contentWidth {
		padStyle := lipgloss.NewStyle().Background(bgColor)
		result.WriteString(padStyle.Render(strings.Repeat(" ", contentWidth-currentLen)))
	}

	return lineNumRendered + " " + result.String()
}

func (m Model) renderSide(lineNum int, content string, lineType git.DiffLineType, width, lineNumWidth int, isCursor bool) string {
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

	// Determine background color based on diff type (subtle tints)
	var bgColor lipgloss.Color
	var defaultFg lipgloss.Color
	switch lineType {
	case git.DiffLineAddition:
		bgColor = lipgloss.Color("#0a1a0a")  // Very subtle dark green
		defaultFg = lipgloss.Color("#88cc88")
	case git.DiffLineDeletion:
		bgColor = lipgloss.Color("#1a0a0a")  // Very subtle dark red
		defaultFg = lipgloss.Color("#cc8888")
	case git.DiffLineHeader:
		bgColor = lipgloss.Color("#0a0a1a")  // Very subtle dark blue
		defaultFg = lipgloss.Color("#8888cc")
	default:
		bgColor = lipgloss.Color("")
		defaultFg = ui.ColorTextMuted
	}

	// Apply syntax highlighting with diff background
	var result strings.Builder
	currentLen := 0

	if m.lexer != nil && m.style != nil && lineType != git.DiffLineHeader {
		iterator, err := m.lexer.Tokenise(nil, displayContent)
		if err == nil {
			for token := iterator(); token != chroma.EOF; token = iterator() {
				tokenText := token.Value

				// Don't exceed codeWidth
				if currentLen+len(tokenText) > codeWidth {
					tokenText = tokenText[:codeWidth-currentLen]
				}
				if len(tokenText) == 0 {
					break
				}

				// Get syntax color from chroma style
				entry := m.style.Get(token.Type)
				style := lipgloss.NewStyle().Background(bgColor)

				if entry.Colour.IsSet() {
					style = style.Foreground(lipgloss.Color(entry.Colour.String()))
				} else {
					style = style.Foreground(defaultFg)
				}
				if entry.Bold == chroma.Yes {
					style = style.Bold(true)
				}
				if entry.Italic == chroma.Yes {
					style = style.Italic(true)
				}

				result.WriteString(style.Render(tokenText))
				currentLen += len(tokenText)

				if currentLen >= codeWidth {
					break
				}
			}
		}
	}

	// If no syntax highlighting was applied, use default styling
	if currentLen == 0 {
		style := lipgloss.NewStyle().Background(bgColor).Foreground(defaultFg)
		result.WriteString(style.Render(displayContent))
		currentLen = len(displayContent)
	}

	// Pad remaining space with background color
	if currentLen < codeWidth {
		padStyle := lipgloss.NewStyle().Background(bgColor)
		result.WriteString(padStyle.Render(strings.Repeat(" ", codeWidth-currentLen)))
	}

	return lineNumRendered + " " + result.String()
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

// GetViewMode returns the current view mode as a string
func (m Model) GetViewMode() string {
	switch m.viewMode {
	case ViewNew:
		return "new"
	case ViewOld:
		return "old"
	default:
		return "both"
	}
}

// GetSearchableLines returns lines for searching based on current view mode
func (m Model) GetSearchableLines() []SearchableLine {
	var result []SearchableLine

	for i, line := range m.lines {
		switch m.viewMode {
		case ViewBoth:
			// Include both sides
			if line.OldContent != "" || line.OldLineNum > 0 {
				result = append(result, SearchableLine{
					LineNum: line.OldLineNum,
					Content: line.OldContent,
					Type:    lineTypeToString(line.OldType),
					OrigIdx: i,
				})
			}
			if line.NewContent != "" || line.NewLineNum > 0 {
				if line.NewContent != line.OldContent || line.NewType != line.OldType {
					result = append(result, SearchableLine{
						LineNum: line.NewLineNum,
						Content: line.NewContent,
						Type:    lineTypeToString(line.NewType),
						OrigIdx: i,
					})
				}
			}
		case ViewNew:
			if line.NewType == git.DiffLineAddition || line.NewType == git.DiffLineContext || line.NewType == git.DiffLineHeader {
				result = append(result, SearchableLine{
					LineNum: line.NewLineNum,
					Content: line.NewContent,
					Type:    lineTypeToString(line.NewType),
					OrigIdx: i,
				})
			}
		case ViewOld:
			if line.OldType == git.DiffLineDeletion || line.OldType == git.DiffLineContext || line.OldType == git.DiffLineHeader {
				result = append(result, SearchableLine{
					LineNum: line.OldLineNum,
					Content: line.OldContent,
					Type:    lineTypeToString(line.OldType),
					OrigIdx: i,
				})
			}
		}
	}

	return result
}

// SearchableLine represents a line that can be searched
type SearchableLine struct {
	LineNum int
	Content string
	Type    string
	OrigIdx int
}

func lineTypeToString(t git.DiffLineType) string {
	switch t {
	case git.DiffLineAddition:
		return "add"
	case git.DiffLineDeletion:
		return "del"
	case git.DiffLineHeader:
		return "header"
	default:
		return "context"
	}
}

// JumpToLine jumps to a specific line index
func (m *Model) JumpToLine(idx int) {
	if idx < 0 || idx >= len(m.lines) {
		return
	}
	m.cursor = idx
	visibleHeight := m.visibleLines()
	// Center the line in view
	m.offset = idx - visibleHeight/2
	if m.offset < 0 {
		m.offset = 0
	}
	if m.offset > len(m.lines)-visibleHeight {
		m.offset = len(m.lines) - visibleHeight
		if m.offset < 0 {
			m.offset = 0
		}
	}
}
