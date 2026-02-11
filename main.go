package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/matthewmyrick/git-diffs/internal/app"
)

func main() {
	baseBranch := flag.String("base", "", "Base branch to compare against (default: main or master)")
	flag.Parse()

	m := app.New(*baseBranch)

	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
