package ui

import "github.com/charmbracelet/lipgloss"

var (
	// Colors
	ColorPrimary    = lipgloss.Color("#7C3AED") // Purple
	ColorSecondary  = lipgloss.Color("#6366F1") // Indigo
	ColorSuccess    = lipgloss.Color("#10B981") // Green
	ColorWarning    = lipgloss.Color("#F59E0B") // Yellow/Orange
	ColorDanger     = lipgloss.Color("#EF4444") // Red
	ColorMuted      = lipgloss.Color("#6B7280") // Gray
	ColorBackground = lipgloss.Color("#1F2937") // Dark gray
	ColorSurface    = lipgloss.Color("#374151") // Lighter dark gray
	ColorText       = lipgloss.Color("#F9FAFB") // White
	ColorTextMuted  = lipgloss.Color("#9CA3AF") // Light gray

	// Header style
	HeaderStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorText).
			Background(ColorPrimary).
			Padding(0, 1)

	// Footer/help style
	FooterStyle = lipgloss.NewStyle().
			Foreground(ColorTextMuted).
			Background(ColorBackground).
			Padding(0, 1)

	// Pane styles
	PaneStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorMuted)

	PaneFocusedStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(ColorPrimary)

	PaneTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorText).
			Padding(0, 1)

	// File status styles
	StatusAddedStyle = lipgloss.NewStyle().
				Foreground(ColorSuccess).
				Bold(true)

	StatusModifiedStyle = lipgloss.NewStyle().
				Foreground(ColorWarning).
				Bold(true)

	StatusDeletedStyle = lipgloss.NewStyle().
				Foreground(ColorDanger).
				Bold(true)

	StatusRenamedStyle = lipgloss.NewStyle().
				Foreground(ColorSecondary).
				Bold(true)

	// File list styles
	FileItemStyle = lipgloss.NewStyle().
			Foreground(ColorText)

	FileItemSelectedStyle = lipgloss.NewStyle().
				Foreground(ColorText).
				Background(ColorSurface)

	// Diff styles
	DiffAdditionStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#22C55E")).
				Background(lipgloss.Color("#14532D"))

	DiffDeletionStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#F87171")).
				Background(lipgloss.Color("#7F1D1D"))

	DiffContextStyle = lipgloss.NewStyle().
				Foreground(ColorTextMuted)

	DiffHeaderStyle = lipgloss.NewStyle().
			Foreground(ColorSecondary).
			Bold(true)

	LineNumberStyle = lipgloss.NewStyle().
			Foreground(ColorMuted).
			Width(4).
			Align(lipgloss.Right)

	// Search styles
	SearchInputStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(ColorPrimary).
				Padding(0, 1)

	SearchResultStyle = lipgloss.NewStyle().
				Foreground(ColorText)

	SearchResultSelectedStyle = lipgloss.NewStyle().
					Foreground(ColorText).
					Background(ColorPrimary)

	SearchMatchStyle = lipgloss.NewStyle().
				Foreground(ColorWarning).
				Bold(true)

	// Error style
	ErrorStyle = lipgloss.NewStyle().
			Foreground(ColorDanger).
			Bold(true)

	// Empty state style
	EmptyStateStyle = lipgloss.NewStyle().
			Foreground(ColorMuted).
			Italic(true)
)
