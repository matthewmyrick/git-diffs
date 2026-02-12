package filelist

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/matthewmyrick/git-diffs/internal/git"
	"github.com/matthewmyrick/git-diffs/internal/ui"
	"github.com/sahilm/fuzzy"
)

// ViewMode represents the current view mode
type ViewMode int

const (
	ViewFolder ViewMode = iota // Files grouped by folder
	ViewType                   // Files grouped by change type
	ViewRaw                    // Flat list
)

// FileSelectMsg is sent when a file is selected with Enter
type FileSelectMsg struct {
	File *git.ChangedFile
}

// Model represents the file list component
type Model struct {
	files        []git.ChangedFile
	displayItems []DisplayItem
	cursor       int
	offset       int
	width        int
	height       int
	focused      bool
	selected     int
	viewMode     ViewMode
	searching    bool
	searchInput  textinput.Model
	searchQuery  string
}

// DisplayItem represents an item in the display list (file or folder header)
type DisplayItem struct {
	IsHeader bool
	Header   string
	File     *git.ChangedFile
}

// New creates a new file list model
func New() Model {
	ti := textinput.New()
	ti.Placeholder = "Search files..."
	ti.CharLimit = 100

	return Model{
		cursor:      0,
		offset:      0,
		selected:    -1,
		viewMode:    ViewFolder,
		searchInput: ti,
	}
}

// SetFiles sets the list of files to display
func (m *Model) SetFiles(files []git.ChangedFile) {
	m.files = files
	m.cursor = 0
	m.offset = 0
	m.searchQuery = ""
	m.rebuildDisplayItems()
	if len(m.displayItems) > 0 {
		// Find first file item (skip headers)
		for i, item := range m.displayItems {
			if !item.IsHeader {
				m.cursor = i
				m.selected = i
				break
			}
		}
	}
}

// SetSize sets the dimensions of the file list
func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.searchInput.Width = width - 6
}

// SetFocused sets whether this component is focused
func (m *Model) SetFocused(focused bool) {
	m.focused = focused
	if !focused {
		m.searching = false
		m.searchInput.Blur()
	}
}

// IsFocused returns whether this component is focused
func (m Model) IsFocused() bool {
	return m.focused
}

// IsSearching returns whether search is active
func (m Model) IsSearching() bool {
	return m.searching
}

// SelectedFile returns the currently selected file
func (m Model) SelectedFile() *git.ChangedFile {
	if m.selected >= 0 && m.selected < len(m.displayItems) {
		item := m.displayItems[m.selected]
		if !item.IsHeader {
			return item.File
		}
	}
	return nil
}

// Files returns all files
func (m Model) Files() []git.ChangedFile {
	return m.files
}

// ViewMode returns the current view mode
func (m Model) ViewMode() ViewMode {
	return m.viewMode
}

func (m Model) viewModeName() string {
	switch m.viewMode {
	case ViewFolder:
		return "Folder"
	case ViewType:
		return "Type"
	case ViewRaw:
		return "Raw"
	}
	return ""
}

// visibleLines returns how many lines can be displayed
func (m Model) visibleLines() int {
	// height - border(2) - title(1) - search(1 if active) - tabs(1)
	extra := 4
	if m.searching {
		extra = 5
	}
	visible := m.height - extra
	if visible < 1 {
		visible = 1
	}
	return visible
}

// rebuildDisplayItems rebuilds the display list based on view mode and search
func (m *Model) rebuildDisplayItems() {
	m.displayItems = nil

	// Filter files if searching
	files := m.files
	if m.searchQuery != "" {
		var paths []string
		for _, f := range m.files {
			paths = append(paths, f.Path)
		}
		matches := fuzzy.Find(m.searchQuery, paths)
		files = nil
		for _, match := range matches {
			files = append(files, m.files[match.Index])
		}
	}

	switch m.viewMode {
	case ViewFolder:
		m.buildFolderView(files)
	case ViewType:
		m.buildTypeView(files)
	case ViewRaw:
		m.buildRawView(files)
	}
}

func (m *Model) buildFolderView(files []git.ChangedFile) {
	// Group by directory
	folders := make(map[string][]git.ChangedFile)
	var folderOrder []string

	for _, f := range files {
		dir := filepath.Dir(f.Path)
		if dir == "." {
			dir = "/"
		}
		if _, exists := folders[dir]; !exists {
			folderOrder = append(folderOrder, dir)
		}
		folders[dir] = append(folders[dir], f)
	}

	sort.Strings(folderOrder)

	for _, dir := range folderOrder {
		m.displayItems = append(m.displayItems, DisplayItem{
			IsHeader: true,
			Header:   dir,
		})
		for i := range folders[dir] {
			m.displayItems = append(m.displayItems, DisplayItem{
				File: &folders[dir][i],
			})
		}
	}
}

func (m *Model) buildTypeView(files []git.ChangedFile) {
	// Group by status
	types := map[git.FileStatus][]git.ChangedFile{
		git.StatusModified: {},
		git.StatusAdded:    {},
		git.StatusDeleted:  {},
	}

	for _, f := range files {
		switch f.Status {
		case git.StatusModified:
			types[git.StatusModified] = append(types[git.StatusModified], f)
		case git.StatusAdded:
			types[git.StatusAdded] = append(types[git.StatusAdded], f)
		case git.StatusDeleted:
			types[git.StatusDeleted] = append(types[git.StatusDeleted], f)
		default:
			types[git.StatusModified] = append(types[git.StatusModified], f)
		}
	}

	order := []struct {
		status git.FileStatus
		name   string
	}{
		{git.StatusModified, "Modified"},
		{git.StatusAdded, "Added"},
		{git.StatusDeleted, "Deleted"},
	}

	for _, o := range order {
		if len(types[o.status]) > 0 {
			m.displayItems = append(m.displayItems, DisplayItem{
				IsHeader: true,
				Header:   fmt.Sprintf("%s (%d)", o.name, len(types[o.status])),
			})
			for i := range types[o.status] {
				m.displayItems = append(m.displayItems, DisplayItem{
					File: &types[o.status][i],
				})
			}
		}
	}
}

func (m *Model) buildRawView(files []git.ChangedFile) {
	for i := range files {
		m.displayItems = append(m.displayItems, DisplayItem{
			File: &files[i],
		})
	}
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

	// Handle search input
	if m.searching {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "esc":
				m.searching = false
				m.searchInput.Blur()
				m.searchQuery = ""
				m.searchInput.SetValue("")
				m.rebuildDisplayItems()
				return m, nil
			case "enter":
				m.searching = false
				m.searchInput.Blur()
				// Keep the filter active, just exit search mode
				return m, nil
			case "up", "down":
				// Allow navigation while searching
				m.searching = false
				m.searchInput.Blur()
			default:
				var cmd tea.Cmd
				m.searchInput, cmd = m.searchInput.Update(msg)
				newQuery := m.searchInput.Value()
				if newQuery != m.searchQuery {
					m.searchQuery = newQuery
					m.rebuildDisplayItems()
					m.cursor = 0
					m.offset = 0
					// Find first file
					for i, item := range m.displayItems {
						if !item.IsHeader {
							m.cursor = i
							m.selected = i
							break
						}
					}
				}
				return m, cmd
			}
		}
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		keys := ui.DefaultKeyMap()
		visibleHeight := m.visibleLines()

		switch {
		case msg.String() == "/":
			m.searching = true
			m.searchInput.Focus()
			m.offset = 0
			return m, textinput.Blink

		case key.Matches(msg, keys.BracketLeft):
			// Previous view mode
			if m.viewMode > 0 {
				m.viewMode--
			} else {
				m.viewMode = ViewRaw
			}
			m.rebuildDisplayItems()
			m.cursor = 0
			m.offset = 0
			m.findFirstFile()

		case key.Matches(msg, keys.BracketRight):
			// Next view mode
			if m.viewMode < ViewRaw {
				m.viewMode++
			} else {
				m.viewMode = ViewFolder
			}
			m.rebuildDisplayItems()
			m.cursor = 0
			m.offset = 0
			m.findFirstFile()

		case key.Matches(msg, keys.Up):
			m.moveCursor(-1)

		case key.Matches(msg, keys.Down):
			m.moveCursor(1)

		case key.Matches(msg, keys.Home):
			m.cursor = 0
			m.offset = 0
			m.findFirstFile()

		case key.Matches(msg, keys.End):
			if len(m.displayItems) > 0 {
				m.cursor = len(m.displayItems) - 1
				m.selected = m.cursor
				if m.cursor >= visibleHeight {
					m.offset = m.cursor - visibleHeight + 1
				}
			}

		case key.Matches(msg, keys.PageUp):
			m.cursor -= visibleHeight
			if m.cursor < 0 {
				m.cursor = 0
			}
			m.findNearestFile()
			m.offset = m.cursor

		case key.Matches(msg, keys.PageDown):
			m.cursor += visibleHeight
			if m.cursor >= len(m.displayItems) {
				m.cursor = len(m.displayItems) - 1
			}
			m.findNearestFile()
			if m.cursor >= m.offset+visibleHeight {
				m.offset = m.cursor - visibleHeight + 1
			}

		case key.Matches(msg, keys.Enter):
			if m.selected >= 0 && m.selected < len(m.displayItems) {
				item := m.displayItems[m.selected]
				if !item.IsHeader && item.File != nil {
					return m, func() tea.Msg {
						return FileSelectMsg{File: item.File}
					}
				}
			}
		}
	}

	return m, nil
}

func (m *Model) moveCursor(delta int) {
	visibleHeight := m.visibleLines()
	newCursor := m.cursor + delta

	if newCursor < 0 {
		newCursor = 0
	}
	if newCursor >= len(m.displayItems) {
		newCursor = len(m.displayItems) - 1
	}

	m.cursor = newCursor

	// Skip headers
	if m.cursor >= 0 && m.cursor < len(m.displayItems) && m.displayItems[m.cursor].IsHeader {
		if delta > 0 && m.cursor < len(m.displayItems)-1 {
			m.cursor++
		} else if delta < 0 && m.cursor > 0 {
			m.cursor--
		}
	}

	m.selected = m.cursor

	// Adjust offset
	if m.cursor < m.offset {
		m.offset = m.cursor
	} else if m.cursor >= m.offset+visibleHeight {
		m.offset = m.cursor - visibleHeight + 1
	}
}

func (m *Model) findFirstFile() {
	for i, item := range m.displayItems {
		if !item.IsHeader {
			m.cursor = i
			m.selected = i
			return
		}
	}
}

func (m *Model) findNearestFile() {
	if m.cursor < 0 || m.cursor >= len(m.displayItems) {
		return
	}
	if !m.displayItems[m.cursor].IsHeader {
		m.selected = m.cursor
		return
	}
	// Search forward first
	for i := m.cursor; i < len(m.displayItems); i++ {
		if !m.displayItems[i].IsHeader {
			m.cursor = i
			m.selected = i
			return
		}
	}
	// Then backward
	for i := m.cursor; i >= 0; i-- {
		if !m.displayItems[i].IsHeader {
			m.cursor = i
			m.selected = i
			return
		}
	}
}

// View implements tea.Model
func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}

	innerWidth := m.width - 4
	visibleHeight := m.visibleLines()

	var lines []string

	// Title with file count
	titleText := fmt.Sprintf("FILES (%d)", len(m.files))
	lines = append(lines, ui.PaneTitleStyle.Render(titleText))

	// Tabs
	tabs := m.renderTabs(innerWidth)
	lines = append(lines, tabs)

	// Search bar (if active)
	if m.searching {
		searchBar := m.searchInput.View()
		lines = append(lines, searchBar)
	}

	if len(m.displayItems) == 0 {
		if m.searchQuery != "" {
			lines = append(lines, ui.EmptyStateStyle.Render("No matches"))
		} else {
			lines = append(lines, ui.EmptyStateStyle.Render("No changes"))
		}
	} else {
		// Calculate visible range
		end := m.offset + visibleHeight
		if end > len(m.displayItems) {
			end = len(m.displayItems)
		}

		for i := m.offset; i < end; i++ {
			item := m.displayItems[i]
			if item.IsHeader {
				lines = append(lines, m.renderHeader(item.Header, innerWidth))
			} else {
				lines = append(lines, m.renderFileLine(item.File, i, innerWidth))
			}
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

func (m Model) renderTabs(width int) string {
	modes := []string{"Folder", "Type", "Raw"}
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

	tabLine := strings.Join(tabs, " ")
	hint := lipgloss.NewStyle().Foreground(ui.ColorMuted).Render(" [/]")
	return tabLine + hint
}

func (m Model) renderHeader(header string, width int) string {
	style := lipgloss.NewStyle().
		Bold(true).
		Foreground(ui.ColorSecondary).
		Background(ui.ColorBackground).
		Width(width)
	return style.Render("  " + header)
}

func (m Model) renderFileLine(file *git.ChangedFile, idx int, width int) string {
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

	cursor := "  "
	if idx == m.cursor && m.focused {
		cursor = "> "
	}

	// Show just filename in folder view, full path otherwise
	path := file.Path
	if m.viewMode == ViewFolder {
		path = filepath.Base(file.Path)
	}

	maxPathWidth := width - 6
	if maxPathWidth < 10 {
		maxPathWidth = 10
	}
	if len(path) > maxPathWidth {
		path = "..." + path[len(path)-maxPathWidth+3:]
	}

	line := fmt.Sprintf("%s%s %s", cursor, status, path)

	var style lipgloss.Style
	if idx == m.cursor && m.focused {
		style = ui.FileItemSelectedStyle
	} else if idx == m.selected {
		style = ui.FileItemStyle.Copy().Foreground(ui.ColorTextMuted)
	} else {
		style = ui.FileItemStyle
	}

	return style.Render(line)
}

// Cursor returns the current cursor position
func (m Model) Cursor() int {
	return m.cursor
}

// SetCursor sets the cursor position
func (m *Model) SetCursor(pos int) {
	if pos >= 0 && pos < len(m.displayItems) {
		m.cursor = pos
		m.selected = pos
		visibleHeight := m.visibleLines()
		if m.cursor < m.offset {
			m.offset = m.cursor
		} else if m.cursor >= m.offset+visibleHeight {
			m.offset = m.cursor - visibleHeight + 1
		}
	}
}
