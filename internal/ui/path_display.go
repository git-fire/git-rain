package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/git-rain/git-rain/internal/git"
)

// AbbreviateUserHome formats an absolute path for display: paths under the
// current user's home directory use a ~/ prefix with forward slashes; all
// other paths are shown as an absolute path (also slash-normalized for display).
func AbbreviateUserHome(path string) string {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	absPath = filepath.Clean(absPath)

	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.ToSlash(absPath)
	}
	homeAbs, err := filepath.Abs(home)
	if err != nil {
		return filepath.ToSlash(absPath)
	}
	homeAbs = filepath.Clean(homeAbs)

	rel, err := filepath.Rel(homeAbs, absPath)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return filepath.ToSlash(absPath)
	}
	if rel == "." {
		return "~"
	}
	return "~/" + filepath.ToSlash(rel)
}

// TruncatePath returns a substring of path starting at the given rune offset, limited
// to maxWidth runes. hasLeft/hasRight indicate whether content is hidden on either side.
func TruncatePath(path string, maxWidth, offset int) (visible string, hasLeft, hasRight bool) {
	if maxWidth <= 0 {
		return "", false, false
	}
	runes := []rune(path)
	total := len(runes)
	if total <= maxWidth {
		return path, false, false
	}
	maxOffset := total - maxWidth
	if offset < 0 {
		offset = 0
	}
	if offset > maxOffset {
		offset = maxOffset
	}
	return string(runes[offset : offset+maxWidth]), offset > 0, offset+maxWidth < total
}

// PathWidthFor returns the number of terminal cells available for the scrollable
// path segment in a repo list row. Uses PanelTextWidth and lipgloss cell widths
// so the composed line fits inside the padded panel.
func PathWidthFor(terminalWidth int, repo git.Repository) int {
	inner := PanelTextWidth(terminalWidth)
	if inner < 16 {
		if inner < 10 {
			return 4
		}
		return inner / 3
	}
	remotesInfo := fmt.Sprintf("(%d remotes)", len(repo.Remotes))
	if len(repo.Remotes) == 0 {
		remotesInfo = "(no remotes!)"
	}
	// Row layout: "> [✓] name (‹PATH›)  [mode] remotes 💧 …scroll-hint"
	prefixW := lipgloss.Width(fmt.Sprintf("> [✓] %s (‹", repo.Name))
	if w := lipgloss.Width(fmt.Sprintf("> [ ] %s (‹", repo.Name)); w > prefixW {
		prefixW = w
	}
	suffixW := lipgloss.Width(fmt.Sprintf("›)  [%s] %s", repo.Mode.String(), remotesInfo))
	const reserve = 34
	pw := inner - prefixW - suffixW - reserve
	if pw < 8 {
		pw = 8
	}
	return pw
}
