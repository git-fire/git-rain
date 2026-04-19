package ui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// mainViewHeaderBlock returns inner content before the repo list (rain, title, quote, waiting line).
func (m RepoSelectorModel) mainViewHeaderBlock() string {
	cw := m.contentWidth()
	rainW := RainDisplayWidth(m.windowWidth)
	var s strings.Builder
	if m.rainVisible() {
		s.WriteString(m.rainBg.Render())
		s.WriteString("\n")
		s.WriteString(m.renderRainWaveStrip(rainW))
		s.WriteString("\n\n")
	}
	titleGradient := lipgloss.NewStyle().
		Bold(true).
		Foreground(activeProfile().titleFg).
		Background(activeProfile().titleBg).
		Padding(0, 2)
	title := "🌧️  GIT RAIN — SELECT REPOSITORIES  🌧️"
	if cw <= 0 {
		s.WriteString(titleGradient.Render(title))
	} else {
		s.WriteString(titleGradient.MaxWidth(cw).Render(title))
	}
	s.WriteString("\n\n")
	if m.quoteVisible() {
		s.WriteString(m.renderStartupQuote())
		s.WriteString("\n\n")
	}
	if len(m.repos) == 0 && !m.scanDone {
		s.WriteString(clampCellWidth(unselectedStyle.Render("  Waiting for repositories..."), cw))
		s.WriteString("\n")
	}
	return s.String()
}

// mainViewFooterBlock returns help text plus optional scan panel (same as View tail).
func (m RepoSelectorModel) mainViewFooterBlock() string {
	cw := m.contentWidth()
	configHint := ""
	if m.cfg != nil {
		configHint = "  c  Settings  |  "
	}
	helpText := "\n" +
		"Controls:\n" +
		"  ↑/k, ↓/j  Navigate  |  ←/→  Scroll path  |  space  Toggle selection\n" +
		"  m  Change mode  |  x  Ignore  |  a  Select all  |  n  Select none  |  r  Toggle rain  |  Shift+L  Toggle log panel  |  e  Export logs\n" +
		"  i  View ignored  |  " + configHint + "enter  Confirm  |  q  Quit\n\n" +
		"Icons:\n" +
		"  💧 = Has uncommitted changes\n" +
		"  [✓] = Selected  |  [ ] = Not selected  |  ‹›  = path scrollable"
	var s strings.Builder
	s.WriteString(helpStyle.MaxWidth(cw).Render(helpText))
	statusStyle := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(activeProfile().scanBorder).
		Padding(0, 1)
	statusLine := m.statusIcon + " " + m.statusLine
	if m.logExportPath != "" {
		statusLine += "  |  last export: " + m.logExportPath
	}
	s.WriteString("\n")
	s.WriteString(statusStyle.MaxWidth(cw).Render(statusLine))
	if m.showLogPanel {
		start := 0
		if len(m.logEntries) > 8 {
			start = len(m.logEntries) - 8
		}
		lines := make([]string, 0, len(m.logEntries)-start)
		for _, entry := range m.logEntries[start:] {
			lines = append(lines, fmt.Sprintf("%s [%s] %s", entry.Timestamp.Format("15:04:05"), entry.Level, entry.Description))
		}
		panelBody := strings.Join(lines, "\n")
		if panelBody == "" {
			panelBody = "(no events yet)"
		}
		panelStyle := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(activeProfile().scanBorder).
			Padding(0, 1)
		s.WriteString("\n")
		s.WriteString(panelStyle.MaxWidth(cw).Render(panelBody))
	}
	if m.scanChan != nil || m.scanDisabled {
		s.WriteString("\n")
		s.WriteString(m.renderScanStatus())
	}
	return s.String()
}

// mainViewRepoListBlock builds the scrollable repo list for a given list *capacity*
// (same meaning as legacy repoListVisibleCount: passed to clampScroll as visible).
func (m RepoSelectorModel) mainViewRepoListBlock(capacity int) string {
	cw := m.contentWidth()
	if len(m.repos) == 0 {
		return ""
	}
	if capacity < 1 {
		capacity = 1
	}
	scrollOffset := m.clampScroll(m.scrollOffset, m.cursor, capacity, len(m.repos))
	hasAbove := scrollOffset > 0
	hasBelow := len(m.repos) > scrollOffset+capacity
	indicators := 0
	if hasAbove {
		indicators++
	}
	if hasBelow {
		indicators++
	}
	itemVisible := capacity - indicators
	hadHiddenRows := hasAbove || hasBelow
	indicatorsSuppressed := false
	viewportWarning := "  ⚠ More repos exist, but ↑/↓ indicators are hidden in this terminal size (enlarge window or press r)."
	warningRows := viewportWarningRows(cw, viewportWarning)
	if itemVisible < 1 {
		hasAbove = false
		hasBelow = false
		itemVisible = capacity
		if hadHiddenRows && capacity-warningRows >= 1 {
			indicatorsSuppressed = true
			itemVisible = capacity - warningRows
		}
		if itemVisible < 1 {
			itemVisible = 1
		}
	}
	end := scrollOffset + itemVisible
	if end > len(m.repos) {
		end = len(m.repos)
	}
	var s strings.Builder
	if hasAbove {
		s.WriteString(clampCellWidth(unselectedStyle.Render(fmt.Sprintf("  ↑ %d more", scrollOffset)), cw))
		s.WriteString("\n")
	}
	for i := scrollOffset; i < end; i++ {
		repo := m.repos[i]
		cur := " "
		if m.cursor == i {
			cur = ">"
		}
		checked := "[ ]"
		style := unselectedStyle
		if m.selected[i] {
			checked = "[✓]"
			style = selectedStyle
		}
		dirtyIndicator := ""
		if repo.IsDirty {
			dirtyIndicator = " 💧"
		}
		remotesInfo := fmt.Sprintf("(%d remotes)", len(repo.Remotes))
		if len(repo.Remotes) == 0 {
			remotesInfo = "(no remotes!)"
		}
		parentPath := AbbreviateUserHome(filepath.Dir(repo.Path))
		pWidth := PathWidthFor(m.windowWidth, repo)
		scrollOff := 0
		if m.cursor == i {
			scrollOff = m.pathScrollOffset
		}
		visPath, hasLeft, hasRight := TruncatePath(parentPath, pWidth, scrollOff)
		leftInd, rightInd := " ", " "
		if hasLeft {
			leftInd = "‹"
		}
		if hasRight {
			rightInd = "›"
		}
		scrollHint := ""
		if m.cursor == i && (hasLeft || hasRight) {
			scrollHint = "  " + scrollHintStyle.Render("<< SCROLL PATH >>")
		}
		line := fmt.Sprintf("%s %s %s (%s%s%s)  [%s] %s%s%s",
			cur, checked,
			style.Render(repo.Name),
			leftInd, visPath, rightInd,
			repo.Mode.String(),
			remotesInfo,
			dirtyIndicator,
			scrollHint,
		)
		s.WriteString(clampCellWidth(line, cw))
		s.WriteString("\n")
	}
	if hasBelow {
		below := len(m.repos) - end
		s.WriteString(clampCellWidth(unselectedStyle.Render(fmt.Sprintf("  ↓ %d more", below)), cw))
		s.WriteString("\n")
	}
	if indicatorsSuppressed {
		s.WriteString(clampCellWidth(viewportWarningStyle.Render(viewportWarning), cw))
		s.WriteString("\n")
	}
	return s.String()
}

// mainViewPanelOuterHeight returns total terminal rows for the bordered main panel
// when the repo list uses the given scroll *capacity* (see clampScroll visible).
func (m RepoSelectorModel) mainViewPanelOuterHeight(capacity int) int {
	innerW := PanelBlockWidth(m.windowWidth)
	body := m.mainViewHeaderBlock() + m.mainViewRepoListBlock(capacity) + m.mainViewFooterBlock()
	return lipgloss.Height(renderMainPanelBox(innerW, body))
}

// mainViewMeasuredRepoListCapacity finds the largest capacity such that the full
// panel (header + list + footer with wrapped help and scan box) fits in the window.
// Fixes top border scrolling away when repo count is large and help text wraps.
func (m RepoSelectorModel) mainViewMeasuredRepoListCapacity() int {
	h := m.windowHeight
	if h < 1 {
		return 1
	}
	if len(m.repos) == 0 {
		return 1
	}
	innerW := PanelBlockWidth(m.windowWidth)
	header := m.mainViewHeaderBlock()
	footer := m.mainViewFooterBlock()
	outerHeight := func(capacity int) int {
		body := header + m.mainViewRepoListBlock(capacity) + footer
		return lipgloss.Height(renderMainPanelBox(innerW, body))
	}
	// Binary search largest capacity that fits; best defaults to 1 when even a
	// single row overflows the terminal (still show one row + scroll).
	best := 1
	lo, hi := 1, len(m.repos)
	for lo <= hi {
		mid := (lo + hi) / 2
		if mid < 1 {
			mid = 1
		}
		if outerHeight(mid) <= h {
			best = mid
			lo = mid + 1
		} else {
			hi = mid - 1
		}
	}
	return best
}

// --- Ignored repositories view (same height measurement as main view) ---

func (m RepoSelectorModel) ignoredViewHeaderBlock() string {
	rainW := RainDisplayWidth(m.windowWidth)
	var s strings.Builder
	if m.rainVisible() {
		s.WriteString(m.rainBg.Render())
		s.WriteString("\n")
		s.WriteString(m.renderRainWaveStrip(rainW))
		s.WriteString("\n\n")
	}
	s.WriteString(m.renderIgnoredViewTitle())
	s.WriteString("\n\n")
	if m.quoteVisible() {
		s.WriteString(m.renderStartupQuote())
		s.WriteString("\n\n")
	}
	return s.String()
}

func (m RepoSelectorModel) ignoredViewFooterBlock() string {
	return renderIgnoredViewHelp(m.contentWidth())
}

func (m RepoSelectorModel) ignoredViewListBlock(capacity int) string {
	cw := m.contentWidth()
	if len(m.ignoredEntries) == 0 {
		return clampCellWidth(unselectedStyle.Render("No ignored repositories."), cw) + "\n"
	}
	if capacity < 1 {
		capacity = 1
	}
	scrollOffset := m.clampScroll(m.ignoredScrollOffset, m.ignoredCursor, capacity, len(m.ignoredEntries))
	hasAbove := scrollOffset > 0
	hasBelow := len(m.ignoredEntries) > scrollOffset+capacity
	indicators := 0
	if hasAbove {
		indicators++
	}
	if hasBelow {
		indicators++
	}
	maxPathCols := cw - 4
	if maxPathCols < 0 {
		maxPathCols = 0
	}
	itemVisible := capacity - indicators
	hadHiddenRows := hasAbove || hasBelow
	indicatorsSuppressed := false
	viewportWarning := "  ⚠ More ignored repos exist, but ↑/↓ indicators are hidden in this terminal size."
	warningRows := viewportWarningRows(cw, viewportWarning)
	if itemVisible < 1 {
		hasAbove = false
		hasBelow = false
		itemVisible = capacity
		if hadHiddenRows && capacity-warningRows >= 1 {
			indicatorsSuppressed = true
			itemVisible = capacity - warningRows
		}
		if itemVisible < 1 {
			itemVisible = 1
		}
	}
	end := scrollOffset + itemVisible
	if end > len(m.ignoredEntries) {
		end = len(m.ignoredEntries)
	}
	var s strings.Builder
	if hasAbove {
		s.WriteString(clampCellWidth(unselectedStyle.Render(fmt.Sprintf("  ↑ %d more", scrollOffset)), cw))
		s.WriteString("\n")
	}
	for i := scrollOffset; i < end; i++ {
		e := m.ignoredEntries[i]
		cur := " "
		if m.ignoredCursor == i {
			cur = ">"
		}
		displayPath := AbbreviateUserHome(e.Path)
		if maxPathCols == 0 {
			displayPath = ""
		} else if len([]rune(displayPath)) > maxPathCols {
			displayPath = string([]rune(displayPath)[:maxPathCols-1]) + "…"
		}
		line := fmt.Sprintf("%s %s", cur, displayPath)
		s.WriteString(clampCellWidth(line, cw))
		s.WriteString("\n")
	}
	if hasBelow {
		below := len(m.ignoredEntries) - end
		s.WriteString(clampCellWidth(unselectedStyle.Render(fmt.Sprintf("  ↓ %d more", below)), cw))
		s.WriteString("\n")
	}
	if indicatorsSuppressed {
		s.WriteString(clampCellWidth(viewportWarningStyle.Render(viewportWarning), cw))
		s.WriteString("\n")
	}
	return s.String()
}

func (m RepoSelectorModel) ignoredViewPanelOuterHeight(capacity int) int {
	innerW := PanelBlockWidth(m.windowWidth)
	body := m.ignoredViewHeaderBlock() + m.ignoredViewListBlock(capacity) + m.ignoredViewFooterBlock()
	return lipgloss.Height(renderMainPanelBox(innerW, body))
}

func (m RepoSelectorModel) ignoredMeasuredListCapacity() int {
	h := m.windowHeight
	if h < 1 {
		return 1
	}
	if len(m.ignoredEntries) == 0 {
		return 1
	}
	innerW := PanelBlockWidth(m.windowWidth)
	header := m.ignoredViewHeaderBlock()
	footer := m.ignoredViewFooterBlock()
	outerHeight := func(capacity int) int {
		body := header + m.ignoredViewListBlock(capacity) + footer
		return lipgloss.Height(renderMainPanelBox(innerW, body))
	}
	best := 1
	lo, hi := 1, len(m.ignoredEntries)
	for lo <= hi {
		mid := (lo + hi) / 2
		if mid < 1 {
			mid = 1
		}
		if outerHeight(mid) <= h {
			best = mid
			lo = mid + 1
		} else {
			hi = mid - 1
		}
	}
	return best
}
