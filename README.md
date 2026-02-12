# git-diffs

A terminal UI for viewing git diffs between branches, inspired by GitHub's PR diff view.

![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)
![License](https://img.shields.io/badge/License-MIT-green.svg)

## Features

- **Side-by-side diff view** - See old and new code side by side, just like GitHub
- **Syntax highlighting** - Powered by Chroma for beautiful code highlighting
- **File list with status indicators** - Quickly see what's Added (A), Modified (M), Deleted (D), or Renamed (R)
- **Fuzzy search** - Quickly find files or search content
- **Full-screen TUI** - Immersive terminal experience like lazygit
- **Keyboard-driven** - Navigate entirely with your keyboard

## Installation

### Using Go Install (Recommended)

```bash
go install github.com/matthewmyrick/git-diffs@latest
```

Make sure `$GOPATH/bin` is in your `PATH`:

```bash
# Add to ~/.zshrc or ~/.bashrc
export PATH="$PATH:$(go env GOPATH)/bin"
```

### Building from Source

```bash
git clone https://github.com/matthewmyrick/git-diffs.git
cd git-diffs
go build -o git-diffs .

# Optional: move to a directory in your PATH
sudo mv git-diffs /usr/local/bin/
```

### Homebrew (Coming Soon)

```bash
brew install matthewmyrick/tap/git-diffs
```

## Usage

```bash
# Run in any git repository
# Compares current branch against main/master (auto-detected)
git-diffs

# Compare against a specific base branch
git-diffs --base develop

# Compare against a specific commit
git-diffs --base HEAD~5
```

## Keyboard Shortcuts

### File List (Left Pane)

| Key | Action |
|-----|--------|
| `↑` / `k` | Move up |
| `↓` / `j` | Move down |
| `Enter` | Select file and view diff |
| `[` / `]` | Switch view mode (Folder / Type / Raw) |
| `/` | Search files (fuzzy) |
| `Esc` | Clear search |

### Diff View (Right Pane)

| Key | Action |
|-----|--------|
| `↑` / `k` | Scroll up |
| `↓` / `j` | Scroll down |
| `Esc` | Return to file list |

### Global

| Key | Action |
|-----|--------|
| `←` / `→` | Switch between panes |
| `q` / `Ctrl+C` | Quit |
| `PgUp` / `Ctrl+U` | Page up |
| `PgDn` / `Ctrl+D` | Page down |
| `Home` / `g` | Go to top |
| `End` / `G` | Go to bottom |

## View Modes

The file list supports three view modes (switch with `[` and `]`):

- **Folder** (default) - Files grouped by directory
- **Type** - Files grouped by change type (Modified, Added, Deleted)
- **Raw** - Flat list of all files

## Requirements

- Go 1.21 or higher
- Git

## Built With

- [Bubble Tea](https://github.com/charmbracelet/bubbletea) - TUI framework
- [Lip Gloss](https://github.com/charmbracelet/lipgloss) - Style definitions
- [Chroma](https://github.com/alecthomas/chroma) - Syntax highlighting
- [Fuzzy](https://github.com/sahilm/fuzzy) - Fuzzy search

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## Roadmap

- [ ] GitHub PR integration (fetch diffs from remote PRs)
- [ ] Inline comments view
- [ ] File tree view
- [ ] Custom themes
- [ ] Configuration file support

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Acknowledgments

- Inspired by [lazygit](https://github.com/jesseduffield/lazygit) and GitHub's PR interface
- Built with the amazing [Charm](https://charm.sh/) libraries
