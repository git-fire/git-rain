package ui

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/git-rain/git-rain/internal/config"
)

type colorProfile struct {
	rain         []lipgloss.Color
	cloudColor   lipgloss.Color
	flowerColor  lipgloss.Color
	titleFg      lipgloss.Color
	titleBg      lipgloss.Color
	selected     lipgloss.Color
	unselected   lipgloss.Color
	help         lipgloss.Color
	scrollHint   lipgloss.Color
	viewportWarn lipgloss.Color
	boxBorder    lipgloss.Color
	scanBorder   lipgloss.Color
	scanWarn     lipgloss.Color
	scanDone     lipgloss.Color
	configCursor lipgloss.Color
	configLabel  lipgloss.Color
	configValue  lipgloss.Color
	configDim    lipgloss.Color
}

var profileMap = map[string]colorProfile{
	config.UIColorProfileStorm: {
		rain: []lipgloss.Color{
			lipgloss.Color("#0A2A4A"),
			lipgloss.Color("#0D3B66"),
			lipgloss.Color("#1565C0"),
			lipgloss.Color("#1976D2"),
			lipgloss.Color("#2196F3"),
			lipgloss.Color("#42A5F5"),
			lipgloss.Color("#90CAF9"),
			lipgloss.Color("#E3F2FD"),
		},
		cloudColor:   lipgloss.Color("#607D8B"),
		flowerColor:  lipgloss.Color("#66BB6A"),
		titleFg:      lipgloss.Color("#42A5F5"),
		titleBg:      lipgloss.Color("#0A1520"),
		selected:     lipgloss.Color("#4DD0E1"),
		unselected:   lipgloss.Color("#78909C"),
		help:         lipgloss.Color("#546E7A"),
		scrollHint:   lipgloss.Color("#80DEEA"),
		viewportWarn: lipgloss.Color("#FFB74D"),
		boxBorder:    lipgloss.Color("#1976D2"),
		scanBorder:   lipgloss.Color("#37474F"),
		scanWarn:     lipgloss.Color("#FFB74D"),
		scanDone:     lipgloss.Color("#4DB6AC"),
		configCursor: lipgloss.Color("#42A5F5"),
		configLabel:  lipgloss.Color("#B0BEC5"),
		configValue:  lipgloss.Color("#80DEEA"),
		configDim:    lipgloss.Color("#546E7A"),
	},
	config.UIColorProfileDrizzle: {
		rain: []lipgloss.Color{
			lipgloss.Color("#90A4AE"),
			lipgloss.Color("#B0BEC5"),
			lipgloss.Color("#CFD8DC"),
			lipgloss.Color("#ECEFF1"),
			lipgloss.Color("#E3F2FD"),
			lipgloss.Color("#BBDEFB"),
			lipgloss.Color("#E1F5FE"),
			lipgloss.Color("#F5F5F5"),
		},
		cloudColor:   lipgloss.Color("#B0BEC5"),
		flowerColor:  lipgloss.Color("#AED581"),
		titleFg:      lipgloss.Color("#546E7A"),
		titleBg:      lipgloss.Color("#ECEFF1"),
		selected:     lipgloss.Color("#0288D1"),
		unselected:   lipgloss.Color("#78909C"),
		help:         lipgloss.Color("#90A4AE"),
		scrollHint:   lipgloss.Color("#0288D1"),
		viewportWarn: lipgloss.Color("#F57C00"),
		boxBorder:    lipgloss.Color("#90A4AE"),
		scanBorder:   lipgloss.Color("#B0BEC5"),
		scanWarn:     lipgloss.Color("#F57C00"),
		scanDone:     lipgloss.Color("#43A047"),
		configCursor: lipgloss.Color("#0288D1"),
		configLabel:  lipgloss.Color("#455A64"),
		configValue:  lipgloss.Color("#0288D1"),
		configDim:    lipgloss.Color("#90A4AE"),
	},
	config.UIColorProfileMonsoon: {
		rain: []lipgloss.Color{
			lipgloss.Color("#002171"),
			lipgloss.Color("#003c8f"),
			lipgloss.Color("#0d47a1"),
			lipgloss.Color("#1565c0"),
			lipgloss.Color("#0277bd"),
			lipgloss.Color("#01579b"),
			lipgloss.Color("#006064"),
			lipgloss.Color("#004d40"),
		},
		cloudColor:   lipgloss.Color("#263238"),
		flowerColor:  lipgloss.Color("#1B5E20"),
		titleFg:      lipgloss.Color("#26C6DA"),
		titleBg:      lipgloss.Color("#000A12"),
		selected:     lipgloss.Color("#00BFA5"),
		unselected:   lipgloss.Color("#455A64"),
		help:         lipgloss.Color("#37474F"),
		scrollHint:   lipgloss.Color("#00BFA5"),
		viewportWarn: lipgloss.Color("#FF6F00"),
		boxBorder:    lipgloss.Color("#006064"),
		scanBorder:   lipgloss.Color("#1C313A"),
		scanWarn:     lipgloss.Color("#FF6F00"),
		scanDone:     lipgloss.Color("#00BFA5"),
		configCursor: lipgloss.Color("#26C6DA"),
		configLabel:  lipgloss.Color("#78909C"),
		configValue:  lipgloss.Color("#00BFA5"),
		configDim:    lipgloss.Color("#37474F"),
	},
	config.UIColorProfileRainbow: {
		rain: []lipgloss.Color{
			lipgloss.Color("#EF5350"),
			lipgloss.Color("#FFA726"),
			lipgloss.Color("#FFEE58"),
			lipgloss.Color("#66BB6A"),
			lipgloss.Color("#42A5F5"),
			lipgloss.Color("#7E57C2"),
			lipgloss.Color("#EC407A"),
			lipgloss.Color("#FFFFFF"),
		},
		cloudColor:   lipgloss.Color("#9E9E9E"),
		flowerColor:  lipgloss.Color("#F48FB1"),
		titleFg:      lipgloss.Color("#FFEE58"),
		titleBg:      lipgloss.Color("#1A1A2E"),
		selected:     lipgloss.Color("#66FF66"),
		unselected:   lipgloss.Color("#9E9E9E"),
		help:         lipgloss.Color("#757575"),
		scrollHint:   lipgloss.Color("#FFEE58"),
		viewportWarn: lipgloss.Color("#FF7043"),
		boxBorder:    lipgloss.Color("#7E57C2"),
		scanBorder:   lipgloss.Color("#424242"),
		scanWarn:     lipgloss.Color("#FF7043"),
		scanDone:     lipgloss.Color("#66BB6A"),
		configCursor: lipgloss.Color("#FFEE58"),
		configLabel:  lipgloss.Color("#E0E0E0"),
		configValue:  lipgloss.Color("#66FF66"),
		configDim:    lipgloss.Color("#757575"),
	},
	config.UIColorProfileSynthwave: {
		rain: []lipgloss.Color{
			lipgloss.Color("#2E1065"),
			lipgloss.Color("#5B21B6"),
			lipgloss.Color("#7C3AED"),
			lipgloss.Color("#A21CAF"),
			lipgloss.Color("#DB2777"),
			lipgloss.Color("#F43F5E"),
			lipgloss.Color("#FB7185"),
			lipgloss.Color("#FDE047"),
		},
		cloudColor:   lipgloss.Color("#6D28D9"),
		flowerColor:  lipgloss.Color("#F472B6"),
		titleFg:      lipgloss.Color("#F472B6"),
		titleBg:      lipgloss.Color("#130A2A"),
		selected:     lipgloss.Color("#22D3EE"),
		unselected:   lipgloss.Color("#A78BFA"),
		help:         lipgloss.Color("#8B5CF6"),
		scrollHint:   lipgloss.Color("#FDE047"),
		viewportWarn: lipgloss.Color("#FB7185"),
		boxBorder:    lipgloss.Color("#C026D3"),
		scanBorder:   lipgloss.Color("#7E22CE"),
		scanWarn:     lipgloss.Color("#FB7185"),
		scanDone:     lipgloss.Color("#22D3EE"),
		configCursor: lipgloss.Color("#F472B6"),
		configLabel:  lipgloss.Color("#E9D5FF"),
		configValue:  lipgloss.Color("#67E8F9"),
		configDim:    lipgloss.Color("#8B5CF6"),
	},
}

var activeRainColors = profileMap[config.UIColorProfileStorm].rain
var activeProfileName = config.UIColorProfileStorm

func applyColorProfile(profile string) string {
	p, ok := profileMap[profile]
	if !ok {
		profile = config.UIColorProfileStorm
		p = profileMap[profile]
	}
	activeProfileName = profile
	activeRainColors = p.rain

	selectedStyle = lipgloss.NewStyle().Foreground(p.selected).Bold(true)
	unselectedStyle = lipgloss.NewStyle().Foreground(p.unselected)
	helpStyle = lipgloss.NewStyle().Foreground(p.help).MarginTop(1)
	scrollHintStyle = lipgloss.NewStyle().Foreground(p.scrollHint).Bold(true)
	viewportWarningStyle = lipgloss.NewStyle().Foreground(p.viewportWarn).Bold(true)
	boxStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(p.boxBorder).Padding(1, 2)

	return profile
}

func activeProfile() colorProfile {
	if p, ok := profileMap[activeProfileName]; ok {
		return p
	}
	return colorProfile{}
}
