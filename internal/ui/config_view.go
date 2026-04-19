package ui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/git-rain/git-rain/internal/config"
)

type configRow struct {
	label   string
	kind    configRowKind
	options []string
}

type configRowKind int

const (
	configRowBool configRowKind = iota
	configRowEnum
	configRowComingSoon
)

var configRows = []configRow{
	{label: "Default mode", kind: configRowEnum, options: []string{
		"sync-default",
		"sync-all",
		"sync-current-branch",
		"leave-untouched",
	}},
	{label: "Disable scan", kind: configRowBool},
	{label: "Fetch workers", kind: configRowEnum, options: []string{
		"1", "2", "4", "8", "16",
	}},
	{label: "Show rain animation", kind: configRowBool},
	{label: "Rain animation mode", kind: configRowEnum, options: []string{
		config.UIRainAnimationBasic,
		config.UIRainAnimationAdvanced,
		config.UIRainAnimationMatrix,
		config.UIRainAnimationGarden,
	}},
	{label: "Show flavor quotes", kind: configRowBool},
	{label: "Flavor quote behavior", kind: configRowEnum, options: []string{
		config.UIQuoteBehaviorRefresh,
		config.UIQuoteBehaviorHide,
	}},
	{label: "Flavor quote interval (s)", kind: configRowEnum, options: []string{
		"5", "10", "15", "30",
	}},
	{label: "Rain speed (ms)", kind: configRowEnum, options: []string{
		"80", "120", "150", "180", "220", "280", "340",
	}},
	{label: "Color profile", kind: configRowEnum, options: config.UIColorProfiles()},
	{label: "Custom hex palette", kind: configRowComingSoon},
}

// Garden settings rows appear in the menu only when rain mode is garden,
// directly under "Rain animation mode" (see logicalRowIndex).
var gardenSettingsConfigRows = []configRow{
	{label: "Garden growth pace", kind: configRowEnum, options: []string{"calm", "normal", "fast"}},
	{label: "Garden seed rate", kind: configRowEnum, options: []string{"rare", "normal", "often"}},
	{label: "Garden offspring", kind: configRowEnum, options: []string{"few", "default", "many"}},
}

func gardenSettingsRowCount(cfg *config.Config) int {
	if cfg != nil && strings.EqualFold(strings.TrimSpace(cfg.UI.RainAnimationMode), config.UIRainAnimationGarden) {
		return len(gardenSettingsConfigRows)
	}
	return 0
}

func visibleConfigRowCount(cfg *config.Config) int {
	return len(configRows) + gardenSettingsRowCount(cfg)
}

// logicalRowIndex maps a visible settings row to legacy ids 0..len(configRows)-1
// or len(configRows)+k for garden-only rows.
func logicalRowIndex(visibleI int, cfg *config.Config) int {
	g := gardenSettingsRowCount(cfg)
	if g == 0 {
		return visibleI
	}
	if visibleI < 5 {
		return visibleI
	}
	if visibleI < 5+g {
		return len(configRows) + (visibleI - 5)
	}
	return visibleI - g
}

func configRowAt(visibleI int, cfg *config.Config) configRow {
	li := logicalRowIndex(visibleI, cfg)
	if li < len(configRows) {
		return configRows[li]
	}
	gi := li - len(configRows)
	if gi >= 0 && gi < len(gardenSettingsConfigRows) {
		return gardenSettingsConfigRows[gi]
	}
	return configRows[len(configRows)-1]
}

func clampConfigCursor(cfg *config.Config, cur int) int {
	n := visibleConfigRowCount(cfg)
	if n <= 0 {
		return 0
	}
	if cur >= n {
		return n - 1
	}
	if cur < 0 {
		return 0
	}
	return cur
}

func gardenGrowthPaceLabel(cfg *config.Config) string {
	if cfg == nil {
		return "normal"
	}
	p := cfg.UI.GardenGrowthPace
	if p <= 0 {
		return "normal"
	}
	if p >= 1.2 {
		return "calm"
	}
	if p < 0.9 {
		return "fast"
	}
	return "normal"
}

func gardenSeedRateLabel(cfg *config.Config) string {
	if cfg == nil {
		return "normal"
	}
	s := cfg.UI.GardenSeedRate
	if s <= 0 {
		return "normal"
	}
	if s < 0.08 {
		return "rare"
	}
	if s > 0.12 {
		return "often"
	}
	return "normal"
}

func gardenOffspringLabel(cfg *config.Config) string {
	if cfg == nil {
		return "default"
	}
	if cfg.UI.GardenOffspringMin <= 0 && cfg.UI.GardenOffspringMax <= 0 {
		return "default"
	}
	if cfg.UI.GardenOffspringMin >= 3 {
		return "many"
	}
	if cfg.UI.GardenOffspringMax <= 2 && cfg.UI.GardenOffspringMin > 0 {
		return "few"
	}
	return "default"
}

func configRowValue(visibleI int, cfg *config.Config) string {
	if cfg == nil {
		return ""
	}
	switch logicalRowIndex(visibleI, cfg) {
	case 0:
		return cfg.Global.DefaultMode
	case 1:
		if cfg.Global.DisableScan {
			return "true"
		}
		return "false"
	case 2:
		return strconv.Itoa(cfg.Global.FetchWorkers)
	case 3:
		if cfg.UI.ShowRainAnimation {
			return "true"
		}
		return "false"
	case 4:
		if cfg.UI.RainAnimationMode == "" {
			return config.UIRainAnimationBasic
		}
		return cfg.UI.RainAnimationMode
	case 5:
		if cfg.UI.ShowStartupQuote {
			return "true"
		}
		return "false"
	case 6:
		return cfg.UI.StartupQuoteBehavior
	case 7:
		return strconv.Itoa(cfg.UI.StartupQuoteIntervalSec)
	case 8:
		if cfg.UI.RainTickMS <= 0 {
			return strconv.Itoa(config.DefaultUIRainTickMS)
		}
		return strconv.Itoa(cfg.UI.RainTickMS)
	case 9:
		return cfg.UI.ColorProfile
	case 10:
		return "coming soon"
	case 11:
		return gardenGrowthPaceLabel(cfg)
	case 12:
		return gardenSeedRateLabel(cfg)
	case 13:
		return gardenOffspringLabel(cfg)
	}
	return ""
}

func applyConfigChange(visibleI int, cfg *config.Config, dir int) {
	if cfg == nil {
		return
	}
	row := configRowAt(visibleI, cfg)
	li := logicalRowIndex(visibleI, cfg)
	switch row.kind {
	case configRowBool:
		switch li {
		case 1:
			cfg.Global.DisableScan = !cfg.Global.DisableScan
		case 3:
			cfg.UI.ShowRainAnimation = !cfg.UI.ShowRainAnimation
		case 5:
			cfg.UI.ShowStartupQuote = !cfg.UI.ShowStartupQuote
		}
	case configRowEnum:
		opts := row.options
		cur := configRowValue(visibleI, cfg)
		idx := 0
		for j, o := range opts {
			if o == cur {
				idx = j
				break
			}
		}
		idx = (idx + dir + len(opts)) % len(opts)
		switch li {
		case 0:
			cfg.Global.DefaultMode = opts[idx]
		case 2:
			workers, err := strconv.Atoi(opts[idx])
			if err == nil && workers > 0 {
				cfg.Global.FetchWorkers = workers
			}
		case 4:
			cfg.UI.RainAnimationMode = opts[idx]
		case 6:
			cfg.UI.StartupQuoteBehavior = opts[idx]
		case 7:
			sec, err := strconv.Atoi(opts[idx])
			if err == nil && sec > 0 {
				cfg.UI.StartupQuoteIntervalSec = sec
			}
		case 8:
			applyRainTickChange(cfg, opts, dir)
		case 9:
			cfg.UI.ColorProfile = opts[idx]
		case 11:
			switch opts[idx] {
			case "calm":
				cfg.UI.GardenGrowthPace = 1.32
			case "fast":
				cfg.UI.GardenGrowthPace = 0.78
			default:
				cfg.UI.GardenGrowthPace = 0
			}
		case 12:
			switch opts[idx] {
			case "rare":
				cfg.UI.GardenSeedRate = 0.06
			case "often":
				cfg.UI.GardenSeedRate = 0.15
			default:
				cfg.UI.GardenSeedRate = 0
			}
		case 13:
			switch opts[idx] {
			case "few":
				cfg.UI.GardenOffspringMin = 1
				cfg.UI.GardenOffspringMax = 2
				cfg.UI.GardenOffspringSpread = 2
			case "many":
				cfg.UI.GardenOffspringMin = 3
				cfg.UI.GardenOffspringMax = 4
				cfg.UI.GardenOffspringSpread = 5
			default:
				cfg.UI.GardenOffspringMin = 0
				cfg.UI.GardenOffspringMax = 0
				cfg.UI.GardenOffspringSpread = 0
			}
		}
	case configRowComingSoon:
		// reserved
	}
}

func applyRainTickChange(cfg *config.Config, options []string, dir int) {
	if cfg == nil || len(options) == 0 || dir == 0 {
		return
	}
	cur := cfg.UI.RainTickMS
	if cur <= 0 {
		cur = config.DefaultUIRainTickMS
	}

	if dir > 0 {
		for _, opt := range options {
			v, err := strconv.Atoi(opt)
			if err == nil && v > cur {
				cfg.UI.RainTickMS = v
				return
			}
		}
		if v, err := strconv.Atoi(options[0]); err == nil {
			cfg.UI.RainTickMS = v
		}
		return
	}

	for i := len(options) - 1; i >= 0; i-- {
		v, err := strconv.Atoi(options[i])
		if err == nil && v < cur {
			cfg.UI.RainTickMS = v
			return
		}
	}
	if v, err := strconv.Atoi(options[len(options)-1]); err == nil {
		cfg.UI.RainTickMS = v
	}
}

func (m RepoSelectorModel) updateConfigView(msg tea.KeyMsg, cmds []tea.Cmd) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		m.quitting = true
		return m, tea.Quit

	case "c", "esc":
		m.view = repoViewMain
		return m, tea.Batch(cmds...)

	case "up", "k":
		if m.configCursor > 0 {
			m.configCursor--
		}

	case "down", "j":
		if m.configCursor < visibleConfigRowCount(m.cfg)-1 {
			m.configCursor++
		}

	case " ", "right", "l":
		applyConfigChange(m.configCursor, m.cfg, +1)
		m.configCursor = clampConfigCursor(m.cfg, m.configCursor)
		if m.cfg != nil {
			applyColorProfile(m.cfg.UI.ColorProfile)
		}
		m = m.saveConfig()
		m, cmds = m.syncRuntimeFromConfig(cmds)

	case "left", "h":
		applyConfigChange(m.configCursor, m.cfg, -1)
		m.configCursor = clampConfigCursor(m.cfg, m.configCursor)
		if m.cfg != nil {
			applyColorProfile(m.cfg.UI.ColorProfile)
		}
		m = m.saveConfig()
		m, cmds = m.syncRuntimeFromConfig(cmds)
	}

	return m, tea.Batch(cmds...)
}

func (m RepoSelectorModel) saveConfig() RepoSelectorModel {
	if m.cfg == nil || m.cfgPath == "" {
		return m
	}
	if err := config.SaveConfig(m.cfg, m.cfgPath); err != nil {
		m.configSaveErr = err
	} else {
		m.configSaveErr = nil
	}
	return m
}

func (m RepoSelectorModel) viewConfig() string {
	var s strings.Builder
	cw := m.contentWidth()

	if m.rainVisible() {
		rainW := RainDisplayWidth(m.windowWidth)
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
	title := "🌧️  GIT RAIN — SETTINGS"
	if cw <= 0 {
		s.WriteString(titleGradient.Render(title))
	} else {
		s.WriteString(titleGradient.MaxWidth(cw).Render(title))
	}
	s.WriteString("\n\n")

	cursorStyle := lipgloss.NewStyle().Foreground(activeProfile().configCursor).Bold(true)
	labelStyle := lipgloss.NewStyle().Foreground(activeProfile().configLabel)
	valueStyle := lipgloss.NewStyle().Foreground(activeProfile().configValue).Bold(true)
	dimStyle := lipgloss.NewStyle().Foreground(activeProfile().configDim)

	for i := 0; i < visibleConfigRowCount(m.cfg); i++ {
		row := configRowAt(i, m.cfg)
		cur := " "
		if m.configCursor == i {
			cur = ">"
		}

		val := configRowValue(i, m.cfg)

		hintStr := ""
		if m.configCursor == i {
			switch row.kind {
			case configRowBool:
				hintStr = dimStyle.Render("  space to toggle")
			case configRowComingSoon:
				hintStr = dimStyle.Render("  coming soon")
			default:
				if cw >= 88 {
					hintStr = dimStyle.Render("  ←/→ to change")
				} else if cw >= 64 {
					hintStr = dimStyle.Render("  ←/→")
				}
			}
		}

		// Explicit space after ":" so label and value never abut (lipgloss Width
		// on styled segments does not insert separators; Bugbot: "Default mode:sync-default").
		line := fmt.Sprintf("%s  %s %s%s",
			cursorStyle.Render(cur),
			labelStyle.Render(row.label+": "),
			valueStyle.Render(val),
			hintStr,
		)
		s.WriteString(clampCellWidth(line, cw))
		s.WriteString("\n")
	}

	s.WriteString("\n")
	if m.configSaveErr != nil {
		errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF6666"))
		s.WriteString(clampCellWidth(errStyle.Render("⚠️  Save failed: "+m.configSaveErr.Error()), cw))
		s.WriteString("\n")
		helpText := "In-memory settings updated; fix the error above to persist to disk.\n" +
			"Controls:  ↑/k, ↓/j  Navigate  |  space/→  Next value  |  ←  Prev value  |  c/Esc  Back  |  q  Quit"
		s.WriteString(helpStyle.MaxWidth(cw).Render(helpText))
	} else {
		cfgPathStr := m.cfgPath
		if cfgPathStr == "" {
			cfgPathStr = "(config path unknown — changes not saved)"
		} else {
			cfgPathStr = AbbreviateUserHome(cfgPathStr)
		}
		helpText := "Changes saved immediately to " + cfgPathStr + "\n" +
			"Controls:  ↑/k, ↓/j  Navigate  |  space/→  Next value  |  ←  Prev value  |  c/Esc  Back  |  q  Quit"
		s.WriteString(helpStyle.MaxWidth(cw).Render(helpText))
	}

	innerW := PanelBlockWidth(m.windowWidth)
	return renderMainPanelBox(innerW, s.String())
}

func (m RepoSelectorModel) syncRuntimeFromConfig(cmds []tea.Cmd) (RepoSelectorModel, []tea.Cmd) {
	if m.cfg == nil {
		return m, cmds
	}
	wasShowingStartupQuote := m.showStartupQuote
	m.showRain = m.cfg.UI.ShowRainAnimation
	m.rainTick = time.Duration(m.cfg.UI.RainTickMS) * time.Millisecond
	m.rainAnimationMode = m.cfg.UI.RainAnimationMode
	if m.rainBg != nil {
		bgW, h := m.rainBg.Width, m.rainBg.Height
		if m.rainBg.Mode != m.rainAnimationMode {
			m.rainBg = NewRainBackground(bgW, h, m.rainAnimationMode)
		} else {
			m.rainBg.Mode = m.rainAnimationMode
		}
		m.applyGardenTuning(m.rainBg)
	}
	m.showStartupQuote = m.cfg.UI.ShowStartupQuote
	m.startupQuoteBehavior = m.cfg.UI.StartupQuoteBehavior
	m.startupQuoteInterval = time.Duration(m.cfg.UI.StartupQuoteIntervalSec) * time.Second
	if m.showStartupQuote {
		if m.currentStartupQuote == "" {
			m.currentStartupQuote = randomStartupRainQuote()
		}
		if !wasShowingStartupQuote {
			m.startupQuoteVisible = true
		}
		if m.startupQuoteInterval > 0 && !m.quoteTickActive {
			cmds = append(cmds, quoteTickCmd(m.startupQuoteInterval))
			m.quoteTickActive = true
		}
	} else {
		m.startupQuoteVisible = false
		m.quoteTickActive = false
	}
	if m.startupQuoteInterval <= 0 {
		m.quoteTickActive = false
	}
	return m, cmds
}
