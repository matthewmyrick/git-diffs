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
	ViewFolder ViewMode = iota // Files in tree structure
	ViewType                   // Files grouped by change type
	ViewRaw                    // Flat list
)

// FileSelectMsg is sent when a file is selected with Enter
type FileSelectMsg struct {
	File *git.ChangedFile
}

// DisplayItem represents an item in the display list
type DisplayItem struct {
	IsFolder    bool
	IsExpanded  bool
	FolderPath  string
	File        *git.ChangedFile
	Indent      int
	IsTypeHeader bool
	TypeHeader  string
}

// Model represents the file list component
type Model struct {
	files          []git.ChangedFile
	displayItems   []DisplayItem
	expandedDirs   map[string]bool
	cursor         int
	offset         int
	width          int
	height         int
	focused        bool
	selected       int
	viewMode       ViewMode
	searching      bool
	searchInput    textinput.Model
	searchQuery    string
}

// New creates a new file list model
func New() Model {
	ti := textinput.New()
	ti.Placeholder = "Search files..."
	ti.CharLimit = 100

	return Model{
		cursor:       0,
		offset:       0,
		selected:     -1,
		viewMode:     ViewFolder,
		searchInput:  ti,
		expandedDirs: make(map[string]bool),
	}
}

// SetFiles sets the list of files to display
func (m *Model) SetFiles(files []git.ChangedFile) {
	m.files = files
	m.cursor = 0
	m.offset = 0
	m.searchQuery = ""

	// Expand all directories by default
	m.expandedDirs = make(map[string]bool)
	for _, f := range files {
		parts := strings.Split(filepath.Dir(f.Path), string(filepath.Separator))
		path := ""
		for _, part := range parts {
			if part == "." {
				continue
			}
			if path == "" {
				path = part
			} else {
				path = path + string(filepath.Separator) + part
			}
			m.expandedDirs[path] = true
		}
	}

	m.rebuildDisplayItems()
	m.findFirstFile()
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
		if !item.IsFolder && !item.IsTypeHeader && item.File != nil {
			return item.File
		}
	}
	return nil
}

// Files returns all files
func (m Model) Files() []git.ChangedFile {
	return m.files
}

// visibleLines returns how many lines can be displayed
func (m Model) visibleLines() int {
	// height - border(2) - title(1) - tabs(1) - search(1)
	visible := m.height - 5
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
		// Remove spaces from query to allow "greptile client" to match "greptile_client"
		query := strings.ReplaceAll(m.searchQuery, " ", "")

		var paths []string
		for _, f := range m.files {
			paths = append(paths, f.Path)
		}
		matches := fuzzy.Find(query, paths)
		files = nil
		for _, match := range matches {
			files = append(files, m.files[match.Index])
		}
	}

	switch m.viewMode {
	case ViewFolder:
		m.buildTreeView(files)
	case ViewType:
		m.buildTypeView(files)
	case ViewRaw:
		m.buildRawView(files)
	}
}

// TreeNode represents a node in the file tree
type TreeNode struct {
	Name     string
	Path     string
	IsDir    bool
	File     *git.ChangedFile
	Children map[string]*TreeNode
}

func (m *Model) buildTreeView(files []git.ChangedFile) {
	// Build tree structure
	root := &TreeNode{
		Name:     "",
		Path:     "",
		IsDir:    true,
		Children: make(map[string]*TreeNode),
	}

	for i := range files {
		f := &files[i]
		parts := strings.Split(f.Path, string(filepath.Separator))
		current := root
		pathSoFar := ""

		for j, part := range parts {
			if pathSoFar == "" {
				pathSoFar = part
			} else {
				pathSoFar = pathSoFar + string(filepath.Separator) + part
			}

			isLast := j == len(parts)-1

			if _, exists := current.Children[part]; !exists {
				current.Children[part] = &TreeNode{
					Name:     part,
					Path:     pathSoFar,
					IsDir:    !isLast,
					Children: make(map[string]*TreeNode),
				}
			}

			if isLast {
				current.Children[part].File = f
				current.Children[part].IsDir = false
			}

			current = current.Children[part]
		}
	}

	// Flatten tree to display items
	m.flattenTree(root, 0)
}

func (m *Model) flattenTree(node *TreeNode, indent int) {
	// Sort children: directories first, then files, both alphabetically
	var dirs, files []string
	for name, child := range node.Children {
		if child.IsDir {
			dirs = append(dirs, name)
		} else {
			files = append(files, name)
		}
	}
	sort.Strings(dirs)
	sort.Strings(files)

	// Add directories
	for _, name := range dirs {
		child := node.Children[name]
		expanded := m.expandedDirs[child.Path]
		m.displayItems = append(m.displayItems, DisplayItem{
			IsFolder:   true,
			IsExpanded: expanded,
			FolderPath: child.Path,
			Indent:     indent,
		})
		if expanded {
			m.flattenTree(child, indent+1)
		}
	}

	// Add files
	for _, name := range files {
		child := node.Children[name]
		m.displayItems = append(m.displayItems, DisplayItem{
			File:   child.File,
			Indent: indent,
		})
	}
}

func (m *Model) buildTypeView(files []git.ChangedFile) {
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
				IsTypeHeader: true,
				TypeHeader:   fmt.Sprintf("%s (%d)", o.name, len(types[o.status])),
			})
			for i := range types[o.status] {
				m.displayItems = append(m.displayItems, DisplayItem{
					File:   &types[o.status][i],
					Indent: 1,
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
				m.findFirstFile()
				return m, nil
			case "enter":
				m.searching = false
				m.searchInput.Blur()
				return m, nil
			case "up", "down":
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
					m.findFirstFile()
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
			if m.cursor >= 0 && m.cursor < len(m.displayItems) {
				item := m.displayItems[m.cursor]
				// Toggle folder expand/collapse
				if item.IsFolder {
					m.expandedDirs[item.FolderPath] = !m.expandedDirs[item.FolderPath]
					m.rebuildDisplayItems()
					// Find the folder again after rebuild
					for i, di := range m.displayItems {
						if di.IsFolder && di.FolderPath == item.FolderPath {
							m.cursor = i
							m.selected = i
							break
						}
					}
				} else if item.File != nil {
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
	if newCursor < 0 {
		newCursor = 0
	}

	m.cursor = newCursor

	// Skip type headers (but not folders - those are selectable)
	if m.cursor >= 0 && m.cursor < len(m.displayItems) && m.displayItems[m.cursor].IsTypeHeader {
		if delta > 0 && m.cursor < len(m.displayItems)-1 {
			m.cursor++
		} else if delta < 0 && m.cursor > 0 {
			m.cursor--
		}
	}

	m.selected = m.cursor

	if m.cursor < m.offset {
		m.offset = m.cursor
	} else if m.cursor >= m.offset+visibleHeight {
		m.offset = m.cursor - visibleHeight + 1
	}
}

func (m *Model) findFirstFile() {
	for i, item := range m.displayItems {
		if !item.IsTypeHeader {
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
	if !m.displayItems[m.cursor].IsTypeHeader {
		m.selected = m.cursor
		return
	}
	for i := m.cursor; i < len(m.displayItems); i++ {
		if !m.displayItems[i].IsTypeHeader {
			m.cursor = i
			m.selected = i
			return
		}
	}
	for i := m.cursor; i >= 0; i-- {
		if !m.displayItems[i].IsTypeHeader {
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

	// Title
	titleText := fmt.Sprintf("FILES (%d)", len(m.files))
	lines = append(lines, ui.PaneTitleStyle.Render(titleText))

	// Tabs
	tabs := m.renderTabs(innerWidth)
	lines = append(lines, tabs)

	// Search bar (always visible)
	searchStyle := lipgloss.NewStyle().Foreground(ui.ColorMuted)
	if m.searching {
		lines = append(lines, m.searchInput.View())
	} else if m.searchQuery != "" {
		lines = append(lines, searchStyle.Render("/ "+m.searchQuery+" (esc to clear)"))
	} else {
		lines = append(lines, searchStyle.Render("/ to search"))
	}

	if len(m.displayItems) == 0 {
		if m.searchQuery != "" {
			lines = append(lines, ui.EmptyStateStyle.Render("No matches"))
		} else {
			lines = append(lines, ui.EmptyStateStyle.Render("No changes"))
		}
	} else {
		end := m.offset + visibleHeight
		if end > len(m.displayItems) {
			end = len(m.displayItems)
		}

		for i := m.offset; i < end; i++ {
			item := m.displayItems[i]
			if item.IsFolder {
				lines = append(lines, m.renderFolderLine(item, i, innerWidth))
			} else if item.IsTypeHeader {
				lines = append(lines, m.renderTypeHeader(item.TypeHeader, innerWidth))
			} else {
				lines = append(lines, m.renderFileLine(item, i, innerWidth))
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

	return strings.Join(tabs, " ")
}

func (m Model) renderFolderLine(item DisplayItem, idx int, width int) string {
	indent := strings.Repeat("  ", item.Indent)

	icon := "▶ "
	if item.IsExpanded {
		icon = "▼ "
	}

	folderName := filepath.Base(item.FolderPath)

	cursor := "  "
	if idx == m.cursor && m.focused {
		cursor = "> "
	}

	line := fmt.Sprintf("%s%s%s%s", cursor, indent, icon, folderName)

	var style lipgloss.Style
	if idx == m.cursor && m.focused {
		style = ui.FileItemSelectedStyle
	} else {
		style = lipgloss.NewStyle().Foreground(ui.ColorSecondary).Bold(true)
	}

	return style.Render(line)
}

func (m Model) renderTypeHeader(header string, width int) string {
	style := lipgloss.NewStyle().
		Bold(true).
		Foreground(ui.ColorSecondary).
		Background(ui.ColorBackground)
	return style.Render("  " + header)
}

func (m Model) renderFileLine(item DisplayItem, idx int, width int) string {
	file := item.File
	if file == nil {
		return ""
	}

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

	indent := strings.Repeat("  ", item.Indent)

	// Show just filename in folder/type view, full path in raw
	path := file.Path
	if m.viewMode == ViewFolder || m.viewMode == ViewType {
		path = filepath.Base(file.Path)
	}

	maxPathWidth := width - 6 - len(indent)
	if maxPathWidth < 10 {
		maxPathWidth = 10
	}
	if len(path) > maxPathWidth {
		path = "..." + path[len(path)-maxPathWidth+3:]
	}

	line := fmt.Sprintf("%s%s%s %s", cursor, indent, status, path)

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
