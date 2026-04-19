// Package ui implements the Bubble Tea TUI for interactive repository selection and theming.
package ui

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/git-rain/git-rain/internal/config"
	"github.com/git-rain/git-rain/internal/git"
	"github.com/git-rain/git-rain/internal/registry"
)

// ErrCancelled is returned by RunRepoSelector when the user cancels the TUI.
var ErrCancelled = errors.New("cancelled")

var (
	selectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#4DD0E1")).
			Bold(true)

	unselectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#78909C"))

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#546E7A")).
			MarginTop(1)

	scrollHintStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#80DEEA")).
			Bold(true)

	viewportWarningStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FFB74D")).
				Bold(true)

	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#1976D2")).
			Padding(1, 2)
)

func init() {
	applyColorProfile(config.UIColorProfileStorm)
}

// ASCII rain frames for animation banner (fallback when rain bg is off)
var rainFrames = []string{
	"  │  │    │  │  │    │",
	"   │  │  │    │  │  │ ",
	"  │   │  │  │   │  │  ",
}

type tickMsg time.Time
type quoteTickMsg time.Time

// pathScrollMsg drives path-marquee advancement at a fixed cadence.
type pathScrollMsg time.Time

// repoDiscoveredMsg is sent when a new repo arrives via the scan channel.
type repoDiscoveredMsg git.Repository

// scanProgressMsg carries the path the scanner is currently visiting.
type scanProgressMsg string

// repoChanDoneMsg is sent when the repo scan channel is closed.
type repoChanDoneMsg struct{}

// progressChanDoneMsg is sent when the folder-progress channel is closed.
type progressChanDoneMsg struct{}

type repoSelectorView int

const (
	repoViewMain repoSelectorView = iota
	repoViewIgnored
	repoViewConfig
)

func tickCmd(interval time.Duration) tea.Cmd {
	return tea.Tick(interval, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func quoteTickCmd(interval time.Duration) tea.Cmd {
	return tea.Tick(interval, func(t time.Time) tea.Msg {
		return quoteTickMsg(t)
	})
}

const pathScrollInterval = 150 * time.Millisecond

func pathScrollCmd() tea.Cmd {
	return tea.Tick(pathScrollInterval, func(t time.Time) tea.Msg {
		return pathScrollMsg(t)
	})
}

func waitForRepo(ch <-chan git.Repository) tea.Cmd {
	return func() tea.Msg {
		repo, ok := <-ch
		if !ok {
			return repoChanDoneMsg{}
		}
		return repoDiscoveredMsg(repo)
	}
}

func waitForProgress(ch <-chan string) tea.Cmd {
	return func() tea.Msg {
		path, ok := <-ch
		if !ok {
			return progressChanDoneMsg{}
		}
		return scanProgressMsg(path)
	}
}

// RepoSelectorModel is the Bubble Tea model for selecting repositories
type RepoSelectorModel struct {
	repos               []git.Repository
	cursor              int
	scrollOffset        int
	ignoredCursor       int
	ignoredScrollOffset int
	view                repoSelectorView
	ignoredEntries      []registry.RegistryEntry
	selected            map[int]bool
	quitting            bool
	confirmed           bool
	frameIndex          int
	rainBg              *RainBackground
	spinner             spinner.Model
	windowWidth         int
	windowHeight        int
	reg                 *registry.Registry
	regPath             string

	pathScrollOffset int
	pathScrollDir    int
	pathScrollPause  int

	scanChan               <-chan git.Repository
	progressChan           <-chan string
	scanDone               bool
	progDone               bool
	scanDisabled           bool
	scanDisabledRunOnly    bool
	scanCurrentPath        string
	scanNewRegistryCount   int
	scanKnownRegistryCount int

	showRain          bool
	rainTick          time.Duration
	rainAnimationMode string

	showStartupQuote     bool
	startupQuoteBehavior string
	startupQuoteInterval time.Duration
	currentStartupQuote  string
	startupQuoteVisible  bool
	quoteTickActive      bool

	cfg           *config.Config
	cfgPath       string
	configCursor  int
	configSaveErr error
}

// NewRepoSelectorModel creates a new repo selector model (static mode).
func NewRepoSelectorModel(repos []git.Repository, reg *registry.Registry, regPath string) RepoSelectorModel {
	applyColorProfile(config.UIColorProfileStorm)
	selected := make(map[int]bool)
	for i := range repos {
		selected[i] = repos[i].Selected
	}

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(activeProfile().boxBorder)

	animMode := config.UIRainAnimationBasic
	rainBg := NewRainBackground(resolveRainBackgroundWidth(80), 5, animMode, nil)

	return RepoSelectorModel{
		repos:                repos,
		cursor:               0,
		selected:             selected,
		rainBg:               rainBg,
		spinner:              s,
		windowWidth:          80,
		windowHeight:         40,
		reg:                  reg,
		regPath:              regPath,
		pathScrollDir:        1,
		showRain:             true,
		rainTick:             time.Duration(config.DefaultUIRainTickMS) * time.Millisecond,
		rainAnimationMode:    animMode,
		showStartupQuote:     true,
		startupQuoteBehavior: config.UIQuoteBehaviorRefresh,
		startupQuoteInterval: time.Duration(config.DefaultUIStartupQuoteIntervalSec) * time.Second,
		currentStartupQuote:  randomStartupRainQuote(),
		startupQuoteVisible:  true,
		quoteTickActive:      true,
	}
}

// NewRepoSelectorModelStream creates a model in streaming mode.
func NewRepoSelectorModelStream(
	scanChan <-chan git.Repository,
	progressChan <-chan string,
	scanDisabled bool,
	scanDisabledRunOnly bool,
	cfg *config.Config,
	cfgPath string,
	reg *registry.Registry,
	regPath string,
) RepoSelectorModel {
	profileName := config.UIColorProfileStorm
	if cfg != nil && cfg.UI.ColorProfile != "" {
		profileName = cfg.UI.ColorProfile
	}
	applyColorProfile(profileName)

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(activeProfile().boxBorder)

	showRain := true
	rainTickMS := config.DefaultUIRainTickMS
	animMode := config.UIRainAnimationBasic
	showStartupQuote := true
	startupQuoteBehavior := config.UIQuoteBehaviorRefresh
	startupQuoteIntervalSec := config.DefaultUIStartupQuoteIntervalSec
	if cfg != nil {
		showRain = cfg.UI.ShowRainAnimation
		if cfg.UI.RainTickMS > 0 {
			rainTickMS = cfg.UI.RainTickMS
		}
		if cfg.UI.RainAnimationMode != "" {
			animMode = cfg.UI.RainAnimationMode
		}
		showStartupQuote = cfg.UI.ShowStartupQuote
		if cfg.UI.StartupQuoteBehavior != "" {
			startupQuoteBehavior = cfg.UI.StartupQuoteBehavior
		}
		if cfg.UI.StartupQuoteIntervalSec > 0 {
			startupQuoteIntervalSec = cfg.UI.StartupQuoteIntervalSec
		}
	}

	rainBg := NewRainBackground(resolveRainBackgroundWidth(80), 5, animMode, cfg)

	return RepoSelectorModel{
		repos:                nil,
		cursor:               0,
		selected:             make(map[int]bool),
		rainBg:               rainBg,
		spinner:              s,
		windowWidth:          80,
		windowHeight:         40,
		reg:                  reg,
		regPath:              regPath,
		scanChan:             scanChan,
		progressChan:         progressChan,
		scanDone:             scanDisabled,
		progDone:             scanDisabled,
		scanDisabled:         scanDisabled,
		scanDisabledRunOnly:  scanDisabledRunOnly,
		cfg:                  cfg,
		cfgPath:              cfgPath,
		showRain:             showRain,
		rainTick:             time.Duration(rainTickMS) * time.Millisecond,
		rainAnimationMode:    animMode,
		showStartupQuote:     showStartupQuote,
		startupQuoteBehavior: startupQuoteBehavior,
		startupQuoteInterval: time.Duration(startupQuoteIntervalSec) * time.Second,
		currentStartupQuote:  randomStartupRainQuote(),
		startupQuoteVisible:  showStartupQuote,
		quoteTickActive:      showStartupQuote && startupQuoteIntervalSec > 0,
	}
}

func (m RepoSelectorModel) Init() tea.Cmd {
	cmds := []tea.Cmd{tickCmd(m.rainTick), pathScrollCmd(), m.spinner.Tick}
	if m.showStartupQuote && m.startupQuoteInterval > 0 {
		cmds = append(cmds, quoteTickCmd(m.startupQuoteInterval))
	}
	if m.scanChan != nil && !m.scanDone {
		cmds = append(cmds, waitForRepo(m.scanChan))
	}
	if m.progressChan != nil && !m.progDone {
		cmds = append(cmds, waitForProgress(m.progressChan))
	}
	return tea.Batch(cmds...)
}

func (m RepoSelectorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case repoDiscoveredMsg:
		repo := git.Repository(msg)
		repo.Selected = true
		idx := len(m.repos)
		m.repos = append(m.repos, repo)
		m.selected[idx] = true
		if repo.IsNewRegistryEntry {
			m.scanNewRegistryCount++
		} else {
			m.scanKnownRegistryCount++
		}
		if m.scanChan != nil {
			cmds = append(cmds, waitForRepo(m.scanChan))
		}

	case scanProgressMsg:
		m.scanCurrentPath = string(msg)
		if m.progressChan != nil && !m.progDone {
			cmds = append(cmds, waitForProgress(m.progressChan))
		}

	case repoChanDoneMsg:
		m.scanDone = true

	case progressChanDoneMsg:
		m.progDone = true

	case tea.WindowSizeMsg:
		m.windowWidth = msg.Width
		m.windowHeight = msg.Height
		bgW := resolveRainBackgroundWidth(msg.Width)
		m.rainBg = NewRainBackground(bgW, 5, m.rainAnimationMode, m.cfg)
		m = m.withClampedPathScroll()
		m.scrollOffset = m.clampScroll(m.scrollOffset, m.cursor, m.repoListVisibleCount(), len(m.repos))
		m.ignoredScrollOffset = m.clampScroll(m.ignoredScrollOffset, m.ignoredCursor, m.ignoredListVisibleCount(), len(m.ignoredEntries))

	case tickMsg:
		m.frameIndex = (m.frameIndex + 1) % len(rainFrames)
		// Advance rain whenever it is enabled — rainVisible() only controls whether
		// the layout has room to paint it; freezing Update() made growth look dead
		// on shorter terminals or when the header was temporarily hidden.
		if m.showRain && m.rainBg != nil {
			m.rainBg.Update()
		}
		return m, tickCmd(m.rainTick)

	case quoteTickMsg:
		m.quoteTickActive = false
		if m.showStartupQuote {
			switch m.startupQuoteBehavior {
			case config.UIQuoteBehaviorHide:
				if m.scanChan != nil && !m.scanDone {
					if m.startupQuoteInterval > 0 {
						cmds = append(cmds, quoteTickCmd(m.startupQuoteInterval))
						m.quoteTickActive = true
					}
				} else {
					m.startupQuoteVisible = false
				}
			default:
				m.currentStartupQuote = randomStartupRainQuote()
				m.startupQuoteVisible = true
				if m.startupQuoteInterval > 0 {
					cmds = append(cmds, quoteTickCmd(m.startupQuoteInterval))
					m.quoteTickActive = true
				}
			}
		}

	case pathScrollMsg:
		m = m.advancePathScroll()
		return m, pathScrollCmd()

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)

	case tea.InterruptMsg:
		m.quitting = true
		return m, tea.Quit

	case tea.KeyMsg:
		if m.view == repoViewConfig {
			return m.updateConfigView(msg, cmds)
		}
		if m.view == repoViewIgnored {
			return m.updateIgnoredView(msg, cmds)
		}
		return m.updateMainView(msg, cmds)
	}

	return m, tea.Batch(cmds...)
}

func (m RepoSelectorModel) updateMainView(msg tea.KeyMsg, cmds []tea.Cmd) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		m.quitting = true
		return m, tea.Quit

	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
			m.pathScrollOffset = 0
			m.pathScrollDir = 1
			m.pathScrollPause = 0
		}
		m.scrollOffset = m.clampScroll(m.scrollOffset, m.cursor, m.repoListVisibleCount(), len(m.repos))

	case "down", "j":
		if m.cursor < len(m.repos)-1 {
			m.cursor++
			m.pathScrollOffset = 0
			m.pathScrollDir = 1
			m.pathScrollPause = 0
		}
		m.scrollOffset = m.clampScroll(m.scrollOffset, m.cursor, m.repoListVisibleCount(), len(m.repos))

	case "left", "h":
		m = m.shiftPathScroll(-1)

	case "right", "l":
		m = m.shiftPathScroll(+1)

	case " ":
		if m.cursor < len(m.repos) {
			m.selected[m.cursor] = !m.selected[m.cursor]
		}

	case "a":
		for i := range m.repos {
			m.selected[i] = true
		}

	case "n":
		for i := range m.repos {
			m.selected[i] = false
		}

	case "m":
		if m.cursor < len(m.repos) {
			m.repos[m.cursor] = cycleRepoMode(m.repos[m.cursor])
			m.persistMode(m.repos[m.cursor].Path, m.repos[m.cursor].Mode)
		}

	case "x":
		if m.cursor < len(m.repos) {
			repoPath := m.repos[m.cursor].Path
			m.persistIgnore(repoPath)
			m.repos = append(m.repos[:m.cursor], m.repos[m.cursor+1:]...)
			// Re-key selected map
			newSelected := make(map[int]bool)
			for k, v := range m.selected {
				if k < m.cursor {
					newSelected[k] = v
				} else if k > m.cursor {
					newSelected[k-1] = v
				}
			}
			m.selected = newSelected
			if m.cursor >= len(m.repos) && m.cursor > 0 {
				m.cursor--
			}
			m.ignoredEntries = IgnoredRegistryEntries(m.reg)
		}

	case "r":
		m.showRain = !m.showRain
		if m.cfg != nil {
			m.cfg.UI.ShowRainAnimation = m.showRain
			m = m.saveConfig()
		}

	case "i":
		m.ignoredEntries = IgnoredRegistryEntries(m.reg)
		m.ignoredCursor = 0
		m.ignoredScrollOffset = 0
		m.view = repoViewIgnored

	case "c":
		if m.cfg != nil {
			m.view = repoViewConfig
		}

	case "enter":
		m.confirmed = true
		m.quitting = true
		return m, tea.Quit
	}

	return m, tea.Batch(cmds...)
}

func (m RepoSelectorModel) updateIgnoredView(msg tea.KeyMsg, cmds []tea.Cmd) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		m.quitting = true
		return m, tea.Quit

	case "esc", "i", "b":
		m.view = repoViewMain

	case "up", "k":
		if m.ignoredCursor > 0 {
			m.ignoredCursor--
		}
		m.ignoredScrollOffset = m.clampScroll(m.ignoredScrollOffset, m.ignoredCursor, m.ignoredListVisibleCount(), len(m.ignoredEntries))

	case "down", "j":
		if m.ignoredCursor < len(m.ignoredEntries)-1 {
			m.ignoredCursor++
		}
		m.ignoredScrollOffset = m.clampScroll(m.ignoredScrollOffset, m.ignoredCursor, m.ignoredListVisibleCount(), len(m.ignoredEntries))

	case "u":
		m = m.restoreIgnoredAtCursor()
	}
	return m, tea.Batch(cmds...)
}

func cycleRepoMode(repo git.Repository) git.Repository {
	switch repo.Mode {
	case git.ModeLeaveUntouched:
		repo.Mode = git.ModeSyncDefault
	case git.ModeSyncDefault:
		repo.Mode = git.ModeSyncAll
	case git.ModeSyncAll:
		repo.Mode = git.ModeSyncCurrentBranch
	case git.ModeSyncCurrentBranch:
		repo.Mode = git.ModeLeaveUntouched
	default:
		repo.Mode = git.ModeSyncDefault
	}
	return repo
}

func (m RepoSelectorModel) rainVisible() bool {
	if !m.showRain {
		return false
	}
	// Need at least enough rows: rain bg (5) + wave (1) + blank (1) + title (1) + list (1) = 9
	return m.windowHeight >= 9
}

func (m RepoSelectorModel) quoteVisible() bool {
	return m.showStartupQuote && m.startupQuoteVisible && m.currentStartupQuote != ""
}

func (m RepoSelectorModel) renderStartupQuote() string {
	quoteStyle := lipgloss.NewStyle().
		Foreground(activeProfile().scrollHint).
		Italic(true)
	cw := m.contentWidth()
	line := "  ☁ " + m.currentStartupQuote
	if cw <= 0 {
		return quoteStyle.Render(line)
	}
	return quoteStyle.MaxWidth(cw).Render(line)
}

func (m RepoSelectorModel) renderIgnoredViewTitle() string {
	titleGradient := lipgloss.NewStyle().
		Bold(true).
		Foreground(activeProfile().titleFg).
		Background(activeProfile().titleBg).
		Padding(0, 2)
	cw := m.contentWidth()
	title := "🌧️  GIT RAIN — IGNORED REPOSITORIES"
	if cw <= 0 {
		return titleGradient.Render(title)
	}
	return titleGradient.MaxWidth(cw).Render(title)
}

func renderIgnoredViewHelp(cw int) string {
	text := "\nControls:\n" +
		"  ↑/k, ↓/j  Navigate  |  u  Restore (un-ignore)  |  b/i/Esc  Back  |  q  Quit"
	if cw <= 0 {
		return helpStyle.Render(text)
	}
	return helpStyle.MaxWidth(cw).Render(text)
}

// repoListVisibleCount returns how many repo rows can fit in the viewport.
// Uses measured lipgloss height (wrapped help, bordered scan panel) so the
// bordered panel never exceeds window height — avoids the top border cropping
// when many repos are listed in a short terminal.
func (m RepoSelectorModel) repoListVisibleCount() int {
	return m.mainViewMeasuredRepoListCapacity()
}

func (m RepoSelectorModel) ignoredListVisibleCount() int {
	return m.ignoredMeasuredListCapacity()
}

// advancePathScroll advances the path scroll for the currently focused row.
func (m RepoSelectorModel) advancePathScroll() RepoSelectorModel {
	if m.cursor >= len(m.repos) {
		return m
	}
	repo := m.repos[m.cursor]
	path := AbbreviateUserHome(filepath.Dir(repo.Path))
	pWidth := PathWidthFor(m.windowWidth, repo)
	runes := []rune(path)
	total := len(runes)
	if total <= pWidth {
		m.pathScrollOffset = 0
		return m
	}
	maxOffset := total - pWidth
	const pauseTicks = 6
	if m.pathScrollPause > 0 {
		m.pathScrollPause--
		return m
	}
	m.pathScrollOffset += m.pathScrollDir
	if m.pathScrollOffset >= maxOffset {
		m.pathScrollOffset = maxOffset
		m.pathScrollDir = -1
		m.pathScrollPause = pauseTicks
	} else if m.pathScrollOffset <= 0 {
		m.pathScrollOffset = 0
		m.pathScrollDir = 1
		m.pathScrollPause = pauseTicks
	}
	return m
}

func (m RepoSelectorModel) shiftPathScroll(delta int) RepoSelectorModel {
	if m.cursor >= len(m.repos) {
		return m
	}
	repo := m.repos[m.cursor]
	path := AbbreviateUserHome(filepath.Dir(repo.Path))
	pWidth := PathWidthFor(m.windowWidth, repo)
	runes := []rune(path)
	maxOffset := len(runes) - pWidth
	if maxOffset < 0 {
		maxOffset = 0
	}
	m.pathScrollOffset += delta
	if m.pathScrollOffset < 0 {
		m.pathScrollOffset = 0
	}
	if m.pathScrollOffset > maxOffset {
		m.pathScrollOffset = maxOffset
	}
	return m
}

func (m RepoSelectorModel) withClampedPathScroll() RepoSelectorModel {
	if m.cursor >= len(m.repos) {
		m.pathScrollOffset = 0
		return m
	}
	repo := m.repos[m.cursor]
	path := AbbreviateUserHome(filepath.Dir(repo.Path))
	pWidth := PathWidthFor(m.windowWidth, repo)
	runes := []rune(path)
	maxOffset := len(runes) - pWidth
	if maxOffset < 0 {
		maxOffset = 0
	}
	if m.pathScrollOffset > maxOffset {
		m.pathScrollOffset = maxOffset
	}
	return m
}

func (m RepoSelectorModel) clampScroll(offset, cursor, visible, total int) int {
	if visible <= 0 || total == 0 {
		return 0
	}
	if offset > total-visible {
		offset = total - visible
	}
	if offset < 0 {
		offset = 0
	}
	itemVisible := visible
	var next int
	for {
		if cursor < offset {
			next = cursor
		} else if cursor >= offset+itemVisible {
			next = cursor - itemVisible + 1
		} else {
			next = offset
		}
		if next == offset {
			break
		}
		offset = next
	}
	return offset
}

func (m RepoSelectorModel) contentWidth() int {
	return PanelTextWidth(m.windowWidth)
}

func resolveRainBackgroundWidth(terminalWidth int) int {
	w := RainDisplayWidth(terminalWidth)
	if w < 1 {
		w = 1
	}
	return w
}

// clampCellWidth keeps one screen row within maxCells using lipgloss truncation.
// Degeneracy: maxCells < 1 means "no usable width" — return s unchanged; empty s
// is also a no-op. For maxCells == 1, truncation still runs (single visible cell).
func clampCellWidth(s string, maxCells int) string {
	if maxCells < 1 || s == "" {
		return s
	}
	return lipgloss.NewStyle().MaxWidth(maxCells).Inline(true).Render(s)
}

func viewportWarningRows(contentWidth int, warning string) int {
	if warning == "" {
		return 1
	}
	if contentWidth < 1 {
		return 1
	}
	h := lipgloss.Height(lipgloss.NewStyle().MaxWidth(contentWidth).Render(warning))
	if h < 1 {
		return 1
	}
	return h
}

func (m RepoSelectorModel) restoreIgnoredAtCursor() RepoSelectorModel {
	if m.reg == nil || m.regPath == "" || len(m.ignoredEntries) == 0 {
		return m
	}
	if m.ignoredCursor < 0 || m.ignoredCursor >= len(m.ignoredEntries) {
		return m
	}
	entry := m.ignoredEntries[m.ignoredCursor]
	absPath, err := filepath.Abs(entry.Path)
	if err != nil {
		return m
	}
	if !m.reg.SetStatus(entry.Path, registry.StatusActive) && !m.reg.SetStatus(absPath, registry.StatusActive) {
		name := entry.Name
		if name == "" {
			name = filepath.Base(absPath)
		}
		m.reg.Upsert(registry.RegistryEntry{
			Path:   absPath,
			Name:   name,
			Status: registry.StatusActive,
			Mode:   entry.Mode,
		})
	}
	_ = registry.Save(m.reg, m.regPath)

	if repo, aerr := git.AnalyzeRepository(absPath); aerr == nil {
		if entry.Mode != "" {
			repo.Mode = git.ParseMode(entry.Mode)
		}
		repo.Selected = true
		if !repoPathInRepos(m.repos, absPath) {
			idx := len(m.repos)
			m.repos = append(m.repos, repo)
			m.selected[idx] = true
		}
	}
	m.ignoredEntries = IgnoredRegistryEntries(m.reg)
	m.ignoredCursor = clampSelectorCursor(m.ignoredCursor, len(m.ignoredEntries))
	return m
}

func clampSelectorCursor(cursor, total int) int {
	if total == 0 {
		return 0
	}
	if cursor >= total {
		return total - 1
	}
	if cursor < 0 {
		return 0
	}
	return cursor
}

func (m RepoSelectorModel) View() string {
	if m.quitting {
		if m.confirmed {
			selectedCount := 0
			for _, sel := range m.selected {
				if sel {
					selectedCount++
				}
			}
			return fmt.Sprintf("\n✅ Selected %d repositories for sync\n\n", selectedCount)
		}
		return "\n❌ Cancelled\n\n"
	}

	if m.view == repoViewIgnored {
		return m.viewIgnoredMain()
	}
	if m.view == repoViewConfig {
		return m.viewConfig()
	}

	visible := m.repoListVisibleCount()
	var s strings.Builder
	s.WriteString(m.mainViewHeaderBlock())
	s.WriteString(m.mainViewRepoListBlock(visible))
	s.WriteString(m.mainViewFooterBlock())

	innerW := PanelBlockWidth(m.windowWidth)
	return renderMainPanelBox(innerW, s.String())
}

func (m RepoSelectorModel) renderScanStatus() string {
	cw := m.contentWidth()
	if cw < 1 {
		cw = 1
	}
	scanStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(activeProfile().scanBorder).
		Padding(0, 1)

	// Full-width, left-aligned row inside the main panel. Do not use clampCellWidth
	// here — it uses Inline(true) and flattens multiline bordered blocks, which
	// breaks the scan box and leaves the label floating toward the center.
	scanRow := func(inner string) string {
		box := scanStyle.Render(inner)
		return lipgloss.NewStyle().Width(cw).Align(lipgloss.Left).Render(box)
	}

	switch {
	case m.scanDisabled:
		var label string
		if m.scanDisabledRunOnly {
			label = "⚠️  Scanning Disabled (this run only)"
		} else {
			label = "⚠️  Scanning Disabled"
		}
		return scanRow(lipgloss.NewStyle().Foreground(activeProfile().scanWarn).Render(label))

	case m.scanDone:
		total := m.scanNewRegistryCount + m.scanKnownRegistryCount
		var msg string
		if total == 0 {
			msg = "✅ Scan Complete  (no repos in list)"
		} else {
			msg = fmt.Sprintf("✅ Scan Complete  (%d in list: %d new to registry, %d known)",
				total, m.scanNewRegistryCount, m.scanKnownRegistryCount)
		}
		return scanRow(lipgloss.NewStyle().Foreground(activeProfile().scanDone).Render(msg))

	default:
		folder := m.scanCurrentPath
		if folder == "" {
			folder = "..."
		}
		maxLen := 50
		if cw > 24 {
			maxLen = cw - 16
			if maxLen < 24 {
				maxLen = 24
			}
		}
		if len(folder) > maxLen {
			folder = "..." + folder[len(folder)-maxLen+3:]
		}
		line1 := fmt.Sprintf("🔍 Scanning: %s", folder)
		total := m.scanNewRegistryCount + m.scanKnownRegistryCount
		line2 := fmt.Sprintf("   In list: %d  (%d new to registry, %d known)",
			total, m.scanNewRegistryCount, m.scanKnownRegistryCount)
		return scanRow(line1 + "\n" + line2)
	}
}

func (m RepoSelectorModel) viewIgnoredMain() string {
	visible := m.ignoredListVisibleCount()
	var s strings.Builder
	s.WriteString(m.ignoredViewHeaderBlock())
	s.WriteString(m.ignoredViewListBlock(visible))
	s.WriteString(m.ignoredViewFooterBlock())

	innerW := PanelBlockWidth(m.windowWidth)
	return renderMainPanelBox(innerW, s.String())
}

func (m RepoSelectorModel) persistMode(repoPath string, mode git.RepoMode) {
	_ = selectorPersistMode(m.reg, m.regPath, repoPath, mode)
}

func (m RepoSelectorModel) persistIgnore(repoPath string) {
	_ = selectorPersistIgnore(m.reg, m.regPath, repoPath)
}

// GetSelectedRepos returns the selected repositories.
func (m RepoSelectorModel) GetSelectedRepos() []git.Repository {
	return selectorGetSelected(m.repos, m.selected)
}

// RunRepoSelector runs the interactive repo selector and returns selected repos.
func RunRepoSelector(repos []git.Repository, reg *registry.Registry, regPath string) ([]git.Repository, error) {
	model := NewRepoSelectorModel(repos, reg, regPath)
	p := tea.NewProgram(model, tea.WithAltScreen())

	finalModel, err := p.Run()
	if err != nil {
		if errors.Is(err, tea.ErrInterrupted) {
			return nil, ErrCancelled
		}
		return nil, err
	}

	m := finalModel.(RepoSelectorModel)
	if !m.confirmed {
		return nil, ErrCancelled
	}

	return m.GetSelectedRepos(), nil
}

// RunRepoSelectorStream runs the interactive repo selector in streaming mode.
func RunRepoSelectorStream(
	scanChan <-chan git.Repository,
	progressChan <-chan string,
	scanDisabled bool,
	scanDisabledRunOnly bool,
	cfg *config.Config,
	cfgPath string,
	reg *registry.Registry,
	regPath string,
) ([]git.Repository, error) {
	model := NewRepoSelectorModelStream(scanChan, progressChan, scanDisabled, scanDisabledRunOnly, cfg, cfgPath, reg, regPath)
	p := tea.NewProgram(model, tea.WithAltScreen())

	finalModel, err := p.Run()
	if err != nil {
		if errors.Is(err, tea.ErrInterrupted) {
			return nil, ErrCancelled
		}
		return nil, err
	}

	m := finalModel.(RepoSelectorModel)
	if !m.confirmed {
		return nil, ErrCancelled
	}

	return m.GetSelectedRepos(), nil
}
