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
)

// Pane represents which pane is currently focused
type Pane int

const (
	PaneFileList Pane = iota
	PaneDiffView
)

// Model is the main application model
type Model struct {
	repo          *git.Repo
	baseBranch    string
	currentBranch string
	files         []git.ChangedFile
	fileList      filelist.Model
	diffView      diffview.Model
	focusedPane   Pane
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
	fl := filelist.New()
	fl.SetFocused(true) // Start with file list focused

	return Model{
		baseBranch:  baseBranch,
		fileList:    fl,
		diffView:    diffview.New(),
		focusedPane: PaneFileList,
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
				baseBranch = "HEAD"
			}
		}

		files, err := repo.GetChangedFiles(baseBranch, "HEAD")
		if err != nil {
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
		// Global quit
		if key.Matches(msg, m.keys.Quit) && !m.fileList.IsSearching() {
			return m, tea.Quit
		}

		// Escape to go back to file list from diff view
		if key.Matches(msg, m.keys.Escape) && m.focusedPane == PaneDiffView {
			m.setFocus(PaneFileList)
			return m, nil
		}

		// Arrow keys for pane switching (only left/right, not up/down)
		if !m.fileList.IsSearching() {
			if key.Matches(msg, m.keys.Right) && m.focusedPane == PaneFileList {
				m.setFocus(PaneDiffView)
				return m, nil
			}
			if key.Matches(msg, m.keys.Left) && m.focusedPane == PaneDiffView {
				m.setFocus(PaneFileList)
				return m, nil
			}
		}

		// Pass to focused pane
		switch m.focusedPane {
		case PaneFileList:
			var cmd tea.Cmd
			m.fileList, cmd = m.fileList.Update(msg)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}

		case PaneDiffView:
			var cmd tea.Cmd
			m.diffView, cmd = m.diffView.Update(msg)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		}

	case filelist.FileSelectMsg:
		// User pressed Enter on a file - load diff and switch to diff pane
		if msg.File != nil {
			m.setFocus(PaneDiffView)
			cmds = append(cmds, m.loadDiff(msg.File.Path))
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
	}

	return m, tea.Batch(cmds...)
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
	if fileListWidth < 25 {
		fileListWidth = 25
	}
	diffViewWidth := m.width - fileListWidth

	m.fileList.SetSize(fileListWidth, contentHeight)
	m.diffView.SetSize(diffViewWidth, contentHeight)
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
	var help string
	if m.focusedPane == PaneFileList {
		help = "↑↓ navigate  [ ] switch view  / search  Enter select  ←→ switch pane  q quit"
	} else {
		help = "↑↓ scroll  ←→ switch pane  Esc back to files  q quit"
	}
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
