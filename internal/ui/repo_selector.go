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
	rainAnimMode        string
	spinner             spinner.Model
	windowWidth         int
	windowHeight        int
	reg                 *registry.Registry
	regPath             string

	pathScrollOffset int
	pathScrollDir    int
	pathScrollPause  int

	scanChan            <-chan git.Repository
	progressChan        <-chan string
	scanDone            bool
	progDone            bool
	scanDisabled        bool
	scanDisabledRunOnly bool
	scanCurrentPath     string
	scanNewRegistryCount   int
	scanKnownRegistryCount int

	showRain             bool
	rainTick             time.Duration
	rainAnimationMode    string

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
	rainBg := NewRainBackground(70, 5, animMode)

	return RepoSelectorModel{
		repos:                repos,
		cursor:               0,
		selected:             selected,
		rainBg:               rainBg,
		rainAnimMode:         animMode,
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

	rainBg := NewRainBackground(70, 5, animMode)

	return RepoSelectorModel{
		repos:                nil,
		cursor:               0,
		selected:             make(map[int]bool),
		rainBg:               rainBg,
		rainAnimMode:         animMode,
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
		bgW := min(msg.Width-4, 70)
		m.rainBg = NewRainBackground(bgW, 5, m.rainAnimationMode)
		m = m.withClampedPathScroll()
		m.scrollOffset = m.clampScroll(m.scrollOffset, m.cursor, m.repoListVisibleCount(), len(m.repos))
		m.ignoredScrollOffset = m.clampScroll(m.ignoredScrollOffset, m.ignoredCursor, m.ignoredListVisibleCount(), len(m.ignoredEntries))

	case tickMsg:
		m.frameIndex = (m.frameIndex + 1) % len(rainFrames)
		if m.rainVisible() {
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
	return quoteStyle.Render("  ☁ " + m.currentStartupQuote)
}

func (m RepoSelectorModel) renderIgnoredViewTitle() string {
	titleGradient := lipgloss.NewStyle().
		Bold(true).
		Foreground(activeProfile().titleFg).
		Background(activeProfile().titleBg).
		Padding(0, 2)
	return titleGradient.Render("🌧️  GIT RAIN — IGNORED REPOSITORIES")
}

func renderIgnoredViewHelp() string {
	return helpStyle.Render(
		"\nControls:\n" +
			"  ↑/k, ↓/j  Navigate  |  u  Restore (un-ignore)  |  b/i/Esc  Back  |  q  Quit",
	)
}

// repoListVisibleCount returns how many repo rows can fit in the viewport.
func (m RepoSelectorModel) repoListVisibleCount() int {
	// Box overhead: 2 border + 2 padding top + 2 padding bottom = 6
	// Rain area: 7 rows (bg 5 + wave 1 + blank 1) when visible
	// Title: 1
	// Blank after title: 1
	// Quote: 1 (when visible)
	// Blank after quote: 1 (when visible)
	// Scan panel: ~3 rows
	// Help: ~5 rows
	// Scroll indicators: up to 2
	overhead := 6 + 1 + 1 // box + title + blank
	if m.rainVisible() {
		overhead += 7
	}
	if m.quoteVisible() {
		overhead += 2
	}
	if m.scanChan != nil || m.scanDisabled {
		overhead += 3
	}
	overhead += 5 // help text
	visible := m.windowHeight - overhead
	if visible < 1 {
		visible = 1
	}
	return visible
}

func (m RepoSelectorModel) ignoredListVisibleCount() int {
	overhead := 6 + 1 + 1 + 3
	if m.rainVisible() {
		overhead += 7
	}
	if m.quoteVisible() {
		overhead += 2
	}
	visible := m.windowHeight - overhead
	if visible < 1 {
		visible = 1
	}
	return visible
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
	w := m.windowWidth - 6
	if w < 0 {
		w = 0
	}
	return w
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

	cw := m.contentWidth()
	rainW := min(cw, 70)

	var s strings.Builder

	if m.rainVisible() {
		s.WriteString(m.rainBg.Render())
		s.WriteString("\n")
		s.WriteString(RenderRainWave(rainW, m.frameIndex, m.rainAnimationMode))
		s.WriteString("\n\n")
	}

	titleGradient := lipgloss.NewStyle().
		Bold(true).
		Foreground(activeProfile().titleFg).
		Background(activeProfile().titleBg).
		Padding(0, 2)
	s.WriteString(titleGradient.Render("🌧️  GIT RAIN — SELECT REPOSITORIES  🌧️"))
	s.WriteString("\n\n")

	if m.quoteVisible() {
		s.WriteString(m.renderStartupQuote())
		s.WriteString("\n\n")
	}

	if len(m.repos) == 0 && !m.scanDone {
		s.WriteString(unselectedStyle.Render("  Waiting for repositories..."))
		s.WriteString("\n")
	}

	visible := m.repoListVisibleCount()
	scrollOffset := m.clampScroll(m.scrollOffset, m.cursor, visible, len(m.repos))

	hasAbove := scrollOffset > 0
	hasBelow := len(m.repos) > scrollOffset+visible
	indicators := 0
	if hasAbove {
		indicators++
	}
	if hasBelow {
		indicators++
	}
	itemVisible := visible - indicators
	hadHiddenRows := hasAbove || hasBelow
	indicatorsSuppressed := false
	viewportWarning := "  ⚠ More repos exist, but ↑/↓ indicators are hidden in this terminal size (enlarge window or press r)."
	warningRows := viewportWarningRows(cw, viewportWarning)
	if itemVisible < 1 {
		hasAbove = false
		hasBelow = false
		itemVisible = visible
		if hadHiddenRows && visible-warningRows >= 1 {
			indicatorsSuppressed = true
			itemVisible = visible - warningRows
		}
		if itemVisible < 1 {
			itemVisible = 1
		}
	}
	end := scrollOffset + itemVisible
	if end > len(m.repos) {
		end = len(m.repos)
	}

	if hasAbove {
		s.WriteString(unselectedStyle.Render(fmt.Sprintf("  ↑ %d more", scrollOffset)))
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
		s.WriteString(line)
		s.WriteString("\n")
	}

	if hasBelow {
		below := len(m.repos) - end
		s.WriteString(unselectedStyle.Render(fmt.Sprintf("  ↓ %d more", below)))
		s.WriteString("\n")
	}
	if indicatorsSuppressed {
		s.WriteString(viewportWarningStyle.Render(viewportWarning))
		s.WriteString("\n")
	}

	configHint := ""
	if m.cfg != nil {
		configHint = "  c  Settings  |  "
	}
	help := helpStyle.Render(
		"\n" +
			"Controls:\n" +
			"  ↑/k, ↓/j  Navigate  |  ←/→  Scroll path  |  space  Toggle selection\n" +
			"  m  Change mode  |  x  Ignore  |  a  Select all  |  n  Select none  |  r  Toggle rain\n" +
			"  i  View ignored  |  " + configHint + "enter  Confirm  |  q  Quit\n\n" +
			"Icons:\n" +
			"  💧 = Has uncommitted changes\n" +
			"  [✓] = Selected  |  [ ] = Not selected  |  ‹›  = path scrollable",
	)
	s.WriteString(help)

	if m.scanChan != nil || m.scanDisabled {
		s.WriteString("\n")
		s.WriteString(m.renderScanStatus())
	}

	innerW := m.windowWidth - 6
	if innerW < 0 {
		innerW = 0
	}
	return boxStyle.Width(innerW).Render(s.String())
}

func (m RepoSelectorModel) renderScanStatus() string {
	scanStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(activeProfile().scanBorder).
		Padding(0, 1)

	switch {
	case m.scanDisabled:
		var label string
		if m.scanDisabledRunOnly {
			label = "⚠️  Scanning Disabled (this run only)"
		} else {
			label = "⚠️  Scanning Disabled"
		}
		return scanStyle.Render(lipgloss.NewStyle().Foreground(activeProfile().scanWarn).Render(label))

	case m.scanDone:
		total := m.scanNewRegistryCount + m.scanKnownRegistryCount
		var msg string
		if total == 0 {
			msg = "✅ Scan Complete  (no repos in list)"
		} else {
			msg = fmt.Sprintf("✅ Scan Complete  (%d in list: %d new to registry, %d known)",
				total, m.scanNewRegistryCount, m.scanKnownRegistryCount)
		}
		return scanStyle.Render(lipgloss.NewStyle().Foreground(activeProfile().scanDone).Render(msg))

	default:
		folder := m.scanCurrentPath
		if folder == "" {
			folder = "..."
		}
		maxLen := 50
		if len(folder) > maxLen {
			folder = "..." + folder[len(folder)-maxLen+3:]
		}
		line1 := fmt.Sprintf("🔍 Scanning: %s", folder)
		total := m.scanNewRegistryCount + m.scanKnownRegistryCount
		line2 := fmt.Sprintf("   In list: %d  (%d new to registry, %d known)",
			total, m.scanNewRegistryCount, m.scanKnownRegistryCount)
		return scanStyle.Render(line1 + "\n" + line2)
	}
}

func (m RepoSelectorModel) viewIgnoredMain() string {
	cw := m.contentWidth()
	rainW := min(cw, 70)

	var s strings.Builder
	if m.rainVisible() {
		s.WriteString(m.rainBg.Render())
		s.WriteString("\n")
		s.WriteString(RenderRainWave(rainW, m.frameIndex, m.rainAnimationMode))
		s.WriteString("\n\n")
	}

	s.WriteString(m.renderIgnoredViewTitle())
	s.WriteString("\n\n")
	if m.quoteVisible() {
		s.WriteString(m.renderStartupQuote())
		s.WriteString("\n\n")
	}

	if len(m.ignoredEntries) == 0 {
		s.WriteString(unselectedStyle.Render("No ignored repositories."))
		s.WriteString("\n")
	} else {
		visible := m.ignoredListVisibleCount()
		scrollOffset := m.clampScroll(m.ignoredScrollOffset, m.ignoredCursor, visible, len(m.ignoredEntries))

		hasAbove := scrollOffset > 0
		hasBelow := len(m.ignoredEntries) > scrollOffset+visible
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

		itemVisible := visible - indicators
		hadHiddenRows := hasAbove || hasBelow
		indicatorsSuppressed := false
		viewportWarning := "  ⚠ More ignored repos exist, but ↑/↓ indicators are hidden in this terminal size."
		warningRows := viewportWarningRows(cw, viewportWarning)
		if itemVisible < 1 {
			hasAbove = false
			hasBelow = false
			itemVisible = visible
			if hadHiddenRows && visible-warningRows >= 1 {
				indicatorsSuppressed = true
				itemVisible = visible - warningRows
			}
			if itemVisible < 1 {
				itemVisible = 1
			}
		}
		end := scrollOffset + itemVisible
		if end > len(m.ignoredEntries) {
			end = len(m.ignoredEntries)
		}

		if hasAbove {
			s.WriteString(unselectedStyle.Render(fmt.Sprintf("  ↑ %d more", scrollOffset)))
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
			fmt.Fprintf(&s, "%s %s\n", cur, displayPath)
		}

		if hasBelow {
			below := len(m.ignoredEntries) - end
			s.WriteString(unselectedStyle.Render(fmt.Sprintf("  ↓ %d more", below)))
			s.WriteString("\n")
		}
		if indicatorsSuppressed {
			s.WriteString(viewportWarningStyle.Render(viewportWarning))
			s.WriteString("\n")
		}
	}

	s.WriteString(renderIgnoredViewHelp())

	innerW := m.windowWidth - 6
	if innerW < 0 {
		innerW = 0
	}
	return boxStyle.Width(innerW).Render(s.String())
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
		return nil, err
	}

	m := finalModel.(RepoSelectorModel)
	if !m.confirmed {
		return nil, ErrCancelled
	}

	return m.GetSelectedRepos(), nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
