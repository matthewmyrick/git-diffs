package git

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// FileStatus represents the type of change for a file
type FileStatus string

const (
	StatusAdded    FileStatus = "A"
	StatusModified FileStatus = "M"
	StatusDeleted  FileStatus = "D"
	StatusRenamed  FileStatus = "R"
	StatusCopied   FileStatus = "C"
	StatusUnknown  FileStatus = "?"
)

// ChangedFile represents a file that has changed between branches
type ChangedFile struct {
	Status   FileStatus
	Path     string
	OldPath  string // Used for renames
	Additions int
	Deletions int
}

// DiffLine represents a single line in a diff
type DiffLine struct {
	Type       DiffLineType
	Content    string
	OldLineNum int
	NewLineNum int
}

// DiffLineType represents the type of diff line
type DiffLineType int

const (
	DiffLineContext DiffLineType = iota
	DiffLineAddition
	DiffLineDeletion
	DiffLineHeader
)

// DiffHunk represents a hunk in a diff
type DiffHunk struct {
	OldStart int
	OldCount int
	NewStart int
	NewCount int
	Lines    []DiffLine
}

// FileDiff represents the diff for a single file
type FileDiff struct {
	OldPath string
	NewPath string
	Hunks   []DiffHunk
}

// Repo represents a git repository
type Repo struct {
	path string
}

// NewRepo creates a new Repo instance for the given path
func NewRepo(path string) (*Repo, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}

	// Check if this is a git repository
	cmd := exec.Command("git", "-C", absPath, "rev-parse", "--git-dir")
	if err := cmd.Run(); err != nil {
		return nil, errors.New("not a git repository")
	}

	return &Repo{path: absPath}, nil
}

// GetCurrentBranch returns the name of the current branch
func (r *Repo) GetCurrentBranch() (string, error) {
	cmd := exec.Command("git", "-C", r.path, "rev-parse", "--abbrev-ref", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get current branch: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// GetDefaultBranch returns the default branch (main or master)
func (r *Repo) GetDefaultBranch() (string, error) {
	// Try main first
	cmd := exec.Command("git", "-C", r.path, "rev-parse", "--verify", "main")
	if err := cmd.Run(); err == nil {
		return "main", nil
	}

	// Try master
	cmd = exec.Command("git", "-C", r.path, "rev-parse", "--verify", "master")
	if err := cmd.Run(); err == nil {
		return "master", nil
	}

	// Fall back to origin/main or origin/master
	cmd = exec.Command("git", "-C", r.path, "rev-parse", "--verify", "origin/main")
	if err := cmd.Run(); err == nil {
		return "origin/main", nil
	}

	cmd = exec.Command("git", "-C", r.path, "rev-parse", "--verify", "origin/master")
	if err := cmd.Run(); err == nil {
		return "origin/master", nil
	}

	return "", errors.New("could not determine default branch")
}

// GetChangedFiles returns a list of files that have changed between base and head
func (r *Repo) GetChangedFiles(base, head string) ([]ChangedFile, error) {
	// Get file list with status
	cmd := exec.Command("git", "-C", r.path, "diff", "--name-status", base+"..."+head)
	out, err := cmd.Output()
	if err != nil {
		// Try without the three-dot notation (for uncommitted changes)
		cmd = exec.Command("git", "-C", r.path, "diff", "--name-status", base)
		out, err = cmd.Output()
		if err != nil {
			return nil, fmt.Errorf("failed to get changed files: %w", err)
		}
	}

	var files []ChangedFile
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}

		status := FileStatus(parts[0][0:1])
		file := ChangedFile{
			Status: status,
			Path:   parts[len(parts)-1],
		}

		// Handle renames (R100 old new)
		if status == StatusRenamed && len(parts) >= 3 {
			file.OldPath = parts[1]
			file.Path = parts[2]
		}

		files = append(files, file)
	}

	// Get stats for additions/deletions
	cmd = exec.Command("git", "-C", r.path, "diff", "--numstat", base+"..."+head)
	out, err = cmd.Output()
	if err != nil {
		cmd = exec.Command("git", "-C", r.path, "diff", "--numstat", base)
		out, _ = cmd.Output()
	}

	statsMap := make(map[string][2]int)
	scanner = bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Fields(line)
		if len(parts) >= 3 {
			adds := 0
			dels := 0
			fmt.Sscanf(parts[0], "%d", &adds)
			fmt.Sscanf(parts[1], "%d", &dels)
			statsMap[parts[2]] = [2]int{adds, dels}
		}
	}

	for i := range files {
		if stats, ok := statsMap[files[i].Path]; ok {
			files[i].Additions = stats[0]
			files[i].Deletions = stats[1]
		}
	}

	return files, nil
}

// GetFileDiff returns the diff for a specific file
func (r *Repo) GetFileDiff(base, head, filePath string) (*FileDiff, error) {
	cmd := exec.Command("git", "-C", r.path, "diff", base+"..."+head, "--", filePath)
	out, err := cmd.Output()
	if err != nil {
		// Try without three-dot notation
		cmd = exec.Command("git", "-C", r.path, "diff", base, "--", filePath)
		out, err = cmd.Output()
		if err != nil {
			return nil, fmt.Errorf("failed to get diff for %s: %w", filePath, err)
		}
	}

	return parseDiff(string(out))
}

// GetFileContent returns the content of a file at a specific ref
func (r *Repo) GetFileContent(ref, filePath string) (string, error) {
	cmd := exec.Command("git", "-C", r.path, "show", ref+":"+filePath)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get file content: %w", err)
	}
	return string(out), nil
}

// HasUncommittedChanges checks if there are uncommitted changes
func (r *Repo) HasUncommittedChanges() (bool, error) {
	cmd := exec.Command("git", "-C", r.path, "status", "--porcelain")
	out, err := cmd.Output()
	if err != nil {
		return false, err
	}
	return len(strings.TrimSpace(string(out))) > 0, nil
}

// parseDiff parses unified diff output into a FileDiff struct
func parseDiff(diffText string) (*FileDiff, error) {
	diff := &FileDiff{}
	lines := strings.Split(diffText, "\n")

	var currentHunk *DiffHunk
	oldLineNum := 0
	newLineNum := 0

	for _, line := range lines {
		if strings.HasPrefix(line, "---") {
			parts := strings.SplitN(line, " ", 2)
			if len(parts) > 1 {
				diff.OldPath = strings.TrimPrefix(parts[1], "a/")
			}
			continue
		}
		if strings.HasPrefix(line, "+++") {
			parts := strings.SplitN(line, " ", 2)
			if len(parts) > 1 {
				diff.NewPath = strings.TrimPrefix(parts[1], "b/")
			}
			continue
		}
		if strings.HasPrefix(line, "@@") {
			// Parse hunk header: @@ -old,count +new,count @@
			if currentHunk != nil {
				diff.Hunks = append(diff.Hunks, *currentHunk)
			}
			currentHunk = &DiffHunk{}

			// Parse the line numbers
			var oldStart, oldCount, newStart, newCount int
			fmt.Sscanf(line, "@@ -%d,%d +%d,%d @@", &oldStart, &oldCount, &newStart, &newCount)
			if oldCount == 0 {
				fmt.Sscanf(line, "@@ -%d +%d,%d @@", &oldStart, &newStart, &newCount)
				oldCount = 1
			}
			if newCount == 0 {
				fmt.Sscanf(line, "@@ -%d,%d +%d @@", &oldStart, &oldCount, &newStart)
				newCount = 1
			}

			currentHunk.OldStart = oldStart
			currentHunk.OldCount = oldCount
			currentHunk.NewStart = newStart
			currentHunk.NewCount = newCount

			oldLineNum = oldStart
			newLineNum = newStart

			currentHunk.Lines = append(currentHunk.Lines, DiffLine{
				Type:    DiffLineHeader,
				Content: line,
			})
			continue
		}
		if currentHunk == nil {
			continue
		}

		if len(line) == 0 {
			// Empty context line
			currentHunk.Lines = append(currentHunk.Lines, DiffLine{
				Type:       DiffLineContext,
				Content:    "",
				OldLineNum: oldLineNum,
				NewLineNum: newLineNum,
			})
			oldLineNum++
			newLineNum++
		} else if line[0] == '+' {
			currentHunk.Lines = append(currentHunk.Lines, DiffLine{
				Type:       DiffLineAddition,
				Content:    line[1:],
				NewLineNum: newLineNum,
			})
			newLineNum++
		} else if line[0] == '-' {
			currentHunk.Lines = append(currentHunk.Lines, DiffLine{
				Type:       DiffLineDeletion,
				Content:    line[1:],
				OldLineNum: oldLineNum,
			})
			oldLineNum++
		} else if line[0] == ' ' {
			currentHunk.Lines = append(currentHunk.Lines, DiffLine{
				Type:       DiffLineContext,
				Content:    line[1:],
				OldLineNum: oldLineNum,
				NewLineNum: newLineNum,
			})
			oldLineNum++
			newLineNum++
		} else if line[0] == '\\' {
			// "\ No newline at end of file" - skip
			continue
		}
	}

	if currentHunk != nil {
		diff.Hunks = append(diff.Hunks, *currentHunk)
	}

	return diff, nil
}

// StatusString returns a human-readable status string
func (s FileStatus) String() string {
	switch s {
	case StatusAdded:
		return "added"
	case StatusModified:
		return "modified"
	case StatusDeleted:
		return "deleted"
	case StatusRenamed:
		return "renamed"
	case StatusCopied:
		return "copied"
	default:
		return "unknown"
	}
}
