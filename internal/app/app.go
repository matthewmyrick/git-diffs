package app

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/matthewmyrick/git-diffs/internal/git"
	"github.com/matthewmyrick/git-diffs/internal/ui"
	"github.com/matthewmyrick/git-diffs/internal/ui/diffview"
	"github.com/matthewmyrick/git-diffs/internal/ui/filelist"
	"github.com/matthewmyrick/git-diffs/internal/ui/search"
)

// Pane represents which pane is currently focused
type Pane int

const (
	PaneFileList Pane = iota
	PaneDiffView
)

// AppState represents the application state
type AppState int

const (
	StateNormal AppState = iota
	StateSearch
)

// Model is the main application model
type Model struct {
	repo          *git.Repo
	baseBranch    string
	currentBranch string
	files         []git.ChangedFile
	fileList      filelist.Model
	diffView      diffview.Model
	search        search.Model
	focusedPane   Pane
	state         AppState
	width         int
	height        int
	err           error
	keys          ui.KeyMap
}

// filesLoadedMsg is sent when files are loaded
type filesLoadedMsg struct {
	files         []git.ChangedFile
	repo          *git.Repo
	baseBranch    string
	currentBranch string
	err           error
}

// diffLoadedMsg is sent when a diff is loaded
type diffLoadedMsg struct {
	diff     *git.FileDiff
	filePath string
	err      error
}

// New creates a new application model
func New(baseBranch string) Model {
	return Model{
		baseBranch:  baseBranch,
		fileList:    filelist.New(),
		diffView:    diffview.New(),
		search:      search.New(),
		focusedPane: PaneFileList,
		state:       StateNormal,
		keys:        ui.DefaultKeyMap(),
	}
}

// Init implements tea.Model
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.loadRepo(),
		tea.EnterAltScreen,
	)
}

func (m Model) loadRepo() tea.Cmd {
	return func() tea.Msg {
		repo, err := git.NewRepo(".")
		if err != nil {
			return filesLoadedMsg{err: err}
		}

		currentBranch, err := repo.GetCurrentBranch()
		if err != nil {
			return filesLoadedMsg{err: err}
		}

		baseBranch := m.baseBranch
		if baseBranch == "" {
			baseBranch, err = repo.GetDefaultBranch()
			if err != nil {
				// If we can't find default branch, try to diff against HEAD
				baseBranch = "HEAD"
			}
		}

		files, err := repo.GetChangedFiles(baseBranch, "HEAD")
		if err != nil {
			// Try diffing uncommitted changes
			files, err = repo.GetChangedFiles(baseBranch, "")
			if err != nil {
				return filesLoadedMsg{err: err}
			}
		}

		return filesLoadedMsg{
			files:         files,
			repo:          repo,
			baseBranch:    baseBranch,
			currentBranch: currentBranch,
		}
	}
}

func (m Model) loadDiff(filePath string) tea.Cmd {
	return func() tea.Msg {
		if m.repo == nil {
			return diffLoadedMsg{err: fmt.Errorf("repository not loaded")}
		}

		diff, err := m.repo.GetFileDiff(m.baseBranch, "HEAD", filePath)
		if err != nil {
			// Try without HEAD
			diff, err = m.repo.GetFileDiff(m.baseBranch, "", filePath)
			if err != nil {
				return diffLoadedMsg{err: err, filePath: filePath}
			}
		}

		return diffLoadedMsg{
			diff:     diff,
			filePath: filePath,
		}
	}
}

// Update implements tea.Model
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.updateLayout()

	case tea.KeyMsg:
		// Handle search mode separately
		if m.state == StateSearch {
			return m.handleSearchInput(msg)
		}

		// Global keys
		switch {
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit

		case key.Matches(msg, m.keys.Search):
			m.state = StateSearch
			m.search.SetMode(search.ModeFile)
			m.search.SetFiles(m.files)
			m.search.SetFocused(true)
			return m, nil

		case key.Matches(msg, m.keys.SearchContent):
			m.state = StateSearch
			m.search.SetMode(search.ModeContent)
			m.search.SetFiles(m.files)
			m.search.SetFocused(true)
			return m, nil

		case key.Matches(msg, m.keys.Tab), key.Matches(msg, m.keys.Right):
			m.cycleFocus(1)

		case key.Matches(msg, m.keys.ShiftTab), key.Matches(msg, m.keys.Left):
			m.cycleFocus(-1)

		case key.Matches(msg, m.keys.Pane1):
			m.setFocus(PaneFileList)

		case key.Matches(msg, m.keys.Pane2):
			m.setFocus(PaneDiffView)

		case key.Matches(msg, m.keys.Enter):
			if m.focusedPane == PaneFileList {
				if file := m.fileList.SelectedFile(); file != nil {
					cmds = append(cmds, m.loadDiff(file.Path))
				}
			}

		default:
			// Pass to focused pane
			switch m.focusedPane {
			case PaneFileList:
				var cmd tea.Cmd
				m.fileList, cmd = m.fileList.Update(msg)
				if cmd != nil {
					cmds = append(cmds, cmd)
				}
				// Auto-load diff when selection changes
				if file := m.fileList.SelectedFile(); file != nil && file.Path != m.diffView.FilePath() {
					cmds = append(cmds, m.loadDiff(file.Path))
				}

			case PaneDiffView:
				var cmd tea.Cmd
				m.diffView, cmd = m.diffView.Update(msg)
				if cmd != nil {
					cmds = append(cmds, cmd)
				}
			}
		}

	case filesLoadedMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.files = msg.files
		m.fileList.SetFiles(m.files)
		m.repo = msg.repo
		m.baseBranch = msg.baseBranch
		m.currentBranch = msg.currentBranch

		// Load first file diff
		if len(m.files) > 0 {
			cmds = append(cmds, m.loadDiff(m.files[0].Path))
		}

	case diffLoadedMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.diffView.SetDiff(msg.diff, msg.filePath)
		m.err = nil

	case search.SelectFileMsg:
		// User selected a file from search
		m.state = StateNormal
		m.search.SetFocused(false)
		m.setFocus(PaneFileList)

		// Find and select the file
		for i, f := range m.files {
			if f.Path == msg.Path {
				m.fileList.SetCursor(i)
				cmds = append(cmds, m.loadDiff(f.Path))
				break
			}
		}
	}

	return m, tea.Batch(cmds...)
}

func (m *Model) handleSearchInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if key.Matches(msg, m.keys.Escape) {
		m.state = StateNormal
		m.search.SetFocused(false)
		return m, nil
	}

	var cmd tea.Cmd
	m.search, cmd = m.search.Update(msg)

	// Check if search was closed
	if !m.search.IsFocused() {
		m.state = StateNormal
	}

	return m, cmd
}

func (m *Model) cycleFocus(direction int) {
	switch m.focusedPane {
	case PaneFileList:
		if direction > 0 {
			m.setFocus(PaneDiffView)
		}
	case PaneDiffView:
		if direction < 0 {
			m.setFocus(PaneFileList)
		}
	}
}

func (m *Model) setFocus(pane Pane) {
	m.focusedPane = pane
	m.fileList.SetFocused(pane == PaneFileList)
	m.diffView.SetFocused(pane == PaneDiffView)
}

func (m *Model) updateLayout() {
	headerHeight := 1
	footerHeight := 1
	contentHeight := m.height - headerHeight - footerHeight - 2

	// File list takes 30% width, diff view takes 70%
	fileListWidth := m.width * 30 / 100
	if fileListWidth < 20 {
		fileListWidth = 20
	}
	diffViewWidth := m.width - fileListWidth

	m.fileList.SetSize(fileListWidth, contentHeight)
	m.diffView.SetSize(diffViewWidth, contentHeight)
	m.search.SetSize(m.width-4, m.height/2)
}

// View implements tea.Model
func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return "Loading..."
	}

	// Error state
	if m.err != nil {
		return m.renderError()
	}

	var b strings.Builder

	// Header
	b.WriteString(m.renderHeader())
	b.WriteString("\n")

	// Main content
	fileListView := m.fileList.View()
	diffViewView := m.diffView.View()

	content := lipgloss.JoinHorizontal(lipgloss.Top, fileListView, diffViewView)
	b.WriteString(content)
	b.WriteString("\n")

	// Footer
	b.WriteString(m.renderFooter())

	// Overlay search if active
	if m.state == StateSearch {
		return m.overlaySearch(b.String())
	}

	return b.String()
}

func (m Model) renderHeader() string {
	branchInfo := fmt.Sprintf("%s → %s", m.currentBranch, m.baseBranch)
	if m.currentBranch == "" {
		branchInfo = "Loading..."
	}

	fileCount := fmt.Sprintf("[%d files changed]", len(m.files))

	title := fmt.Sprintf(" Git Diffs: %s  %s ", branchInfo, fileCount)

	return ui.HeaderStyle.
		Width(m.width).
		Render(title)
}

func (m Model) renderFooter() string {
	help := "↑↓ scroll  Tab switch pane  / search files  \\ search content  Enter select  q quit"
	return ui.FooterStyle.
		Width(m.width).
		Render(help)
}

func (m Model) renderError() string {
	errorBox := ui.ErrorStyle.
		Width(m.width - 4).
		Padding(1, 2).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ui.ColorDanger).
		Render(fmt.Sprintf("Error: %s", m.err.Error()))

	return lipgloss.Place(
		m.width,
		m.height,
		lipgloss.Center,
		lipgloss.Center,
		errorBox,
	)
}

func (m Model) overlaySearch(base string) string {
	searchView := m.search.View()

	// Center the search modal
	return lipgloss.Place(
		m.width,
		m.height,
		lipgloss.Center,
		lipgloss.Center,
		searchView,
		lipgloss.WithWhitespaceChars(" "),
		lipgloss.WithWhitespaceForeground(lipgloss.Color("#000000")),
	)
}
