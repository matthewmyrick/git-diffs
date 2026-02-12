package filepicker

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/matthewmyrick/git-diffs/internal/git"
	"github.com/matthewmyrick/git-diffs/internal/ui"
	"github.com/sahilm/fuzzy"
)

// CloseMsg is sent when the file picker should close
type CloseMsg struct{}

// FileSelectedMsg is sent when a file is selected
type FileSelectedMsg struct {
	File *git.ChangedFile
}

// Model represents the file picker overlay
type Model struct {
	files       []git.ChangedFile
	diffs       map[string]*git.FileDiff // Cache of loaded diffs
	matches     []fuzzy.Match
	searchInput textinput.Model
	cursor      int
	offset      int
	width       int
	height      int
	active      bool
	repo        *git.Repo
	baseBranch  string
}

// New creates a new file picker model
func New() Model {
	ti := textinput.New()
	ti.Placeholder = "Search files..."
	ti.CharLimit = 200
	ti.Width = 40

	return Model{
		searchInput: ti,
		diffs:       make(map[string]*git.FileDiff),
	}
}

// SetFiles sets the list of files
func (m *Model) SetFiles(files []git.ChangedFile) {
	m.files = files
	m.updateMatches()
}

// SetRepo sets the repo for loading diffs
func (m *Model) SetRepo(repo *git.Repo, baseBranch string) {
	m.repo = repo
	m.baseBranch = baseBranch
}

// SetSize sets the overlay dimensions
func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// Open activates the file picker
func (m *Model) Open() {
	m.active = true
	m.searchInput.SetValue("")
	m.searchInput.Focus()
	m.cursor = 0
	m.offset = 0
	m.updateMatches()
}

// Close deactivates the file picker
func (m *Model) Close() {
	m.active = false
	m.searchInput.Blur()
}

// IsActive returns whether the picker is active
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
				file := &m.files[idx]
				m.Close()
				return m, func() tea.Msg { return FileSelectedMsg{File: file} }
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
		m.matches = make([]fuzzy.Match, len(m.files))
		for i := range m.files {
			m.matches[i] = fuzzy.Match{Index: i}
		}
		return
	}

	var paths []string
	for _, f := range m.files {
		paths = append(paths, f.Path)
	}

	m.matches = fuzzy.Find(query, paths)
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
	h := m.overlayHeight() - 6
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

// loadDiff loads and caches a diff for a file
func (m *Model) loadDiff(path string) *git.FileDiff {
	if diff, ok := m.diffs[path]; ok {
		return diff
	}

	if m.repo == nil {
		return nil
	}

	diff, err := m.repo.GetFileDiff(m.baseBranch, "HEAD", path)
	if err != nil {
		diff, err = m.repo.GetFileDiff(m.baseBranch, "", path)
		if err != nil {
			return nil
		}
	}

	m.diffs[path] = diff
	return diff
}

// RenderOverlay renders the file picker on top of a background
func (m Model) RenderOverlay(background string) string {
	if !m.active || m.width == 0 || m.height == 0 {
		return background
	}

	overlayWidth := m.overlayWidth()
	overlayHeight := m.overlayHeight()
	contentHeight := m.contentHeight()

	// Left pane (files) takes 35%, right pane (preview) takes 65%
	leftWidth := (overlayWidth - 6) * 35 / 100
	rightWidth := (overlayWidth - 6) - leftWidth - 3

	// Build left pane content
	var leftLines []string

	// Title
	title := lipgloss.NewStyle().Bold(true).Foreground(ui.ColorPrimary).Render("Files")
	leftLines = append(leftLines, title)

	// Search input
	searchLine := m.renderSearchInput(leftWidth)
	leftLines = append(leftLines, searchLine)
	leftLines = append(leftLines, lipgloss.NewStyle().Foreground(ui.ColorMuted).Render(strings.Repeat("─", leftWidth)))

	// File list
	if len(m.matches) == 0 {
		leftLines = append(leftLines, ui.EmptyStateStyle.Render("No matches"))
	} else {
		end := m.offset + contentHeight - 1 // -1 for title
		if end > len(m.matches) {
			end = len(m.matches)
		}

		for i := m.offset; i < end; i++ {
			match := m.matches[i]
			file := m.files[match.Index]
			leftLines = append(leftLines, m.renderFileLine(file, i == m.cursor, leftWidth, match))
		}
	}

	// Pad to fixed height
	for len(leftLines) < contentHeight+2 {
		leftLines = append(leftLines, strings.Repeat(" ", leftWidth))
	}
	if len(leftLines) > contentHeight+2 {
		leftLines = leftLines[:contentHeight+2]
	}

	// Build right pane content (diff preview)
	var rightLines []string
	rightLines = append(rightLines, lipgloss.NewStyle().Bold(true).Foreground(ui.ColorPrimary).Render("Preview"))
	rightLines = append(rightLines, lipgloss.NewStyle().Foreground(ui.ColorMuted).Render(strings.Repeat("─", rightWidth)))

	if len(m.matches) > 0 && m.cursor < len(m.matches) {
		idx := m.matches[m.cursor].Index
		file := m.files[idx]
		rightLines = append(rightLines, m.renderDiffPreview(file.Path, rightWidth, contentHeight)...)
	} else {
		rightLines = append(rightLines, ui.EmptyStateStyle.Render("Select a file"))
	}

	// Pad to fixed height
	for len(rightLines) < contentHeight+2 {
		rightLines = append(rightLines, strings.Repeat(" ", rightWidth))
	}
	if len(rightLines) > contentHeight+2 {
		rightLines = rightLines[:contentHeight+2]
	}

	// Render panes
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

func (m Model) compositeOverlay(background, overlay string, overlayW, overlayH int) string {
	bgLines := strings.Split(background, "\n")

	for len(bgLines) < m.height {
		bgLines = append(bgLines, "")
	}

	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#444444"))
	for i := range bgLines {
		plain := stripAnsi(bgLines[i])
		if len(plain) < m.width {
			plain = plain + strings.Repeat(" ", m.width-len(plain))
		}
		bgLines[i] = dimStyle.Render(plain)
	}

	startRow := (m.height - overlayH) / 2
	startCol := (m.width - overlayW) / 2

	overlayLines := strings.Split(overlay, "\n")

	for i, overlayLine := range overlayLines {
		bgRow := startRow + i
		if bgRow >= 0 && bgRow < len(bgLines) {
			bgLines[bgRow] = m.insertOverlayLine(bgLines[bgRow], overlayLine, startCol)
		}
	}

	return strings.Join(bgLines[:m.height], "\n")
}

func (m Model) insertOverlayLine(bgLine, overlayLine string, startCol int) string {
	bgRunes := []rune(stripAnsi(bgLine))

	for len(bgRunes) < m.width {
		bgRunes = append(bgRunes, ' ')
	}

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

func (m Model) renderFileLine(file git.ChangedFile, selected bool, width int, match fuzzy.Match) string {
	// Status indicator
	var statusColor lipgloss.Color
	switch file.Status {
	case git.StatusAdded:
		statusColor = ui.ColorSuccess
	case git.StatusDeleted:
		statusColor = ui.ColorDanger
	case git.StatusModified:
		statusColor = ui.ColorWarning
	default:
		statusColor = ui.ColorMuted
	}

	status := lipgloss.NewStyle().Foreground(statusColor).Bold(true).Render(string(file.Status))

	// Path
	path := file.Path
	maxPathWidth := width - 8
	if maxPathWidth < 10 {
		maxPathWidth = 10
	}
	if len(path) > maxPathWidth {
		path = "..." + path[len(path)-maxPathWidth+3:]
	}

	// Highlight matches
	var styledPath string
	if len(match.MatchedIndexes) > 0 && m.searchInput.Value() != "" {
		styledPath = m.highlightMatches(path, match.MatchedIndexes, file.Path)
	} else {
		styledPath = path
	}

	// Cursor
	cursor := "  "
	if selected {
		cursor = lipgloss.NewStyle().Foreground(ui.ColorPrimary).Bold(true).Render("> ")
	}

	lineStr := fmt.Sprintf("%s%s %s", cursor, status, styledPath)

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

func (m Model) highlightMatches(displayPath string, matchedIndexes []int, originalPath string) string {
	// Map matched indexes from original path to display path
	offset := len(originalPath) - len(displayPath)
	if offset < 0 {
		offset = 0
	}

	matchSet := make(map[int]bool)
	for _, idx := range matchedIndexes {
		displayIdx := idx - offset
		if displayIdx >= 0 && displayIdx < len(displayPath) {
			matchSet[displayIdx] = true
		}
	}

	var result strings.Builder
	highlightStyle := lipgloss.NewStyle().Foreground(ui.ColorWarning).Bold(true)

	for i, char := range displayPath {
		if matchSet[i] {
			result.WriteString(highlightStyle.Render(string(char)))
		} else {
			result.WriteRune(char)
		}
	}

	return result.String()
}

func (m *Model) renderDiffPreview(path string, width int, height int) []string {
	var lines []string

	diff := m.loadDiff(path)
	if diff == nil {
		lines = append(lines, ui.EmptyStateStyle.Render("Loading..."))
		return lines
	}

	// Flatten hunks into lines
	var allLines []struct {
		lineNum int
		content string
		typ     git.DiffLineType
	}

	for _, hunk := range diff.Hunks {
		for _, line := range hunk.Lines {
			num := line.NewLineNum
			if num == 0 {
				num = line.OldLineNum
			}
			allLines = append(allLines, struct {
				lineNum int
				content string
				typ     git.DiffLineType
			}{num, line.Content, line.Type})
		}
	}

	// Show first `height` lines
	end := height
	if end > len(allLines) {
		end = len(allLines)
	}

	for i := 0; i < end; i++ {
		line := allLines[i]

		var bgColor lipgloss.Color
		var fgColor lipgloss.Color
		var prefix string

		switch line.typ {
		case git.DiffLineAddition:
			bgColor = lipgloss.Color("#0a1a0a")
			fgColor = lipgloss.Color("#88cc88")
			prefix = "+"
		case git.DiffLineDeletion:
			bgColor = lipgloss.Color("#1a0a0a")
			fgColor = lipgloss.Color("#cc8888")
			prefix = "-"
		case git.DiffLineHeader:
			bgColor = lipgloss.Color("#0a0a1a")
			fgColor = lipgloss.Color("#8888cc")
			prefix = "@"
		default:
			bgColor = lipgloss.Color("")
			fgColor = ui.ColorTextMuted
			prefix = " "
		}

		lineNum := fmt.Sprintf("%4d", line.lineNum)
		if line.lineNum == 0 {
			lineNum = "    "
		}

		content := line.content
		maxWidth := width - 8
		if maxWidth < 5 {
			maxWidth = 5
		}
		if len(content) > maxWidth {
			content = content[:maxWidth-1] + "…"
		}
		if len(content) < maxWidth {
			content = content + strings.Repeat(" ", maxWidth-len(content))
		}

		lineNumStyle := lipgloss.NewStyle().Foreground(ui.ColorMuted)
		contentStyle := lipgloss.NewStyle().Background(bgColor).Foreground(fgColor)

		lines = append(lines, prefix+" "+lineNumStyle.Render(lineNum)+" "+contentStyle.Render(content))
	}

	if len(allLines) > height {
		more := fmt.Sprintf("... +%d more lines", len(allLines)-height)
		lines = append(lines, ui.EmptyStateStyle.Render(more))
	}

	return lines
}

// View returns empty - use RenderOverlay instead
func (m Model) View() string {
	return ""
}
