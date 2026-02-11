package filelist

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/matthewmyrick/git-diffs/internal/git"
	"github.com/matthewmyrick/git-diffs/internal/ui"
)

// Model represents the file list component
type Model struct {
	files    []git.ChangedFile
	cursor   int
	offset   int
	width    int
	height   int
	focused  bool
	selected int // Currently selected file index
}

// New creates a new file list model
func New() Model {
	return Model{
		cursor:   0,
		offset:   0,
		selected: -1,
	}
}

// SetFiles sets the list of files to display
func (m *Model) SetFiles(files []git.ChangedFile) {
	m.files = files
	m.cursor = 0
	m.offset = 0
	if len(files) > 0 {
		m.selected = 0
	}
}

// SetSize sets the dimensions of the file list
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

// SelectedFile returns the currently selected file
func (m Model) SelectedFile() *git.ChangedFile {
	if m.selected >= 0 && m.selected < len(m.files) {
		return &m.files[m.selected]
	}
	return nil
}

// SelectedIndex returns the currently selected index
func (m Model) SelectedIndex() int {
	return m.selected
}

// Files returns all files
func (m Model) Files() []git.ChangedFile {
	return m.files
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
			if m.cursor > 0 {
				m.cursor--
				m.selected = m.cursor
				if m.cursor < m.offset {
					m.offset = m.cursor
				}
			}

		case key.Matches(msg, keys.Down):
			if m.cursor < len(m.files)-1 {
				m.cursor++
				m.selected = m.cursor
				visibleHeight := m.height - 2 // Account for borders
				if m.cursor >= m.offset+visibleHeight {
					m.offset = m.cursor - visibleHeight + 1
				}
			}

		case key.Matches(msg, keys.Home):
			m.cursor = 0
			m.selected = 0
			m.offset = 0

		case key.Matches(msg, keys.End):
			if len(m.files) > 0 {
				m.cursor = len(m.files) - 1
				m.selected = m.cursor
				visibleHeight := m.height - 2
				if m.cursor >= visibleHeight {
					m.offset = m.cursor - visibleHeight + 1
				}
			}

		case key.Matches(msg, keys.PageUp):
			pageSize := m.height - 2
			m.cursor -= pageSize
			if m.cursor < 0 {
				m.cursor = 0
			}
			m.selected = m.cursor
			m.offset -= pageSize
			if m.offset < 0 {
				m.offset = 0
			}

		case key.Matches(msg, keys.PageDown):
			pageSize := m.height - 2
			m.cursor += pageSize
			if m.cursor >= len(m.files) {
				m.cursor = len(m.files) - 1
			}
			if m.cursor < 0 {
				m.cursor = 0
			}
			m.selected = m.cursor
			visibleHeight := m.height - 2
			if m.cursor >= m.offset+visibleHeight {
				m.offset = m.cursor - visibleHeight + 1
			}

		case key.Matches(msg, keys.Enter):
			m.selected = m.cursor
		}
	}

	return m, nil
}

// View implements tea.Model
func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}

	var b strings.Builder

	// Calculate available height for content (minus title)
	contentHeight := m.height - 2 // Border takes 2 lines
	if contentHeight < 1 {
		contentHeight = 1
	}

	// Title
	title := ui.PaneTitleStyle.Render("FILES")

	// Content width (inside borders)
	contentWidth := m.width - 2
	if contentWidth < 10 {
		contentWidth = 10
	}

	// Build file list
	if len(m.files) == 0 {
		emptyMsg := ui.EmptyStateStyle.Render("No changes")
		b.WriteString(emptyMsg)
	} else {
		visibleFiles := m.files
		start := m.offset
		end := m.offset + contentHeight - 1 // -1 for title
		if end > len(m.files) {
			end = len(m.files)
		}
		if start < len(visibleFiles) && end <= len(visibleFiles) {
			visibleFiles = m.files[start:end]
		}

		for i, file := range visibleFiles {
			idx := i + m.offset
			line := m.renderFileLine(file, idx, contentWidth)
			b.WriteString(line)
			if i < len(visibleFiles)-1 {
				b.WriteString("\n")
			}
		}
	}

	// Apply pane style based on focus
	var paneStyle lipgloss.Style
	if m.focused {
		paneStyle = ui.PaneFocusedStyle
	} else {
		paneStyle = ui.PaneStyle
	}

	content := b.String()

	// Pad content to fill height
	lines := strings.Count(content, "\n") + 1
	for i := lines; i < contentHeight-1; i++ {
		content += "\n"
	}

	// Combine title and content
	fullContent := title + "\n" + content

	return paneStyle.
		Width(m.width - 2).
		Height(m.height - 2).
		Render(fullContent)
}

func (m Model) renderFileLine(file git.ChangedFile, idx int, width int) string {
	// Status indicator
	var statusStyle lipgloss.Style
	switch file.Status {
	case git.StatusAdded:
		statusStyle = ui.StatusAddedStyle
	case git.StatusModified:
		statusStyle = ui.StatusModifiedStyle
	case git.StatusDeleted:
		statusStyle = ui.StatusDeletedStyle
	case git.StatusRenamed:
		statusStyle = ui.StatusRenamedStyle
	default:
		statusStyle = lipgloss.NewStyle()
	}

	status := statusStyle.Render(string(file.Status))

	// File path (truncate if needed)
	path := file.Path
	maxPathWidth := width - 4 // Account for status and spacing
	if len(path) > maxPathWidth {
		path = "..." + path[len(path)-maxPathWidth+3:]
	}

	// Stats
	var stats string
	if file.Additions > 0 || file.Deletions > 0 {
		stats = fmt.Sprintf(" +%d -%d", file.Additions, file.Deletions)
	}

	// Combine
	line := fmt.Sprintf("%s %s%s", status, path, stats)

	// Apply selection style
	var style lipgloss.Style
	if idx == m.cursor && m.focused {
		style = ui.FileItemSelectedStyle
	} else if idx == m.selected {
		style = ui.FileItemSelectedStyle.Foreground(ui.ColorTextMuted)
	} else {
		style = ui.FileItemStyle
	}

	return style.Width(width).Render(line)
}

// Cursor returns the current cursor position
func (m Model) Cursor() int {
	return m.cursor
}

// SetCursor sets the cursor position
func (m *Model) SetCursor(pos int) {
	if pos >= 0 && pos < len(m.files) {
		m.cursor = pos
		m.selected = pos
	}
}
