// Package flavor provides small helpers for optional motivational UI copy.
package flavor

var startupRainQuotes = []string{
	"Let it rain.",
	"Bring the cloud down.",
	"Every drop counts — fetch them all.",
	"The forecast: 100% chance of synced branches.",
	"Into every dev's life a little rain must fall.",
	"Some people feel the rain; others just get wet — fetch first.",
	"Make it rain commits.",
	"The cloud is not a place, it's a remote.",
	"April showers bring fast-forwards.",
	"Fill your local with upstream goodness.",
	"Hydrate before you create.",
	"Let the upstream flow through you.",
	"Clean worktree, clear skies.",
	"Pull from the source, not from memory.",
	"Every great branch starts with a good fetch.",
	"The only bad sync is the one that didn't happen.",
	"Your origin misses you.",
	"Refresh, rehydrate, refactor.",
	"Water the branches; watch them grow.",
	"Storms pass; commits are forever.",
	"Remotes are just friends you haven't fetched yet.",
	"Don't let your local branch wither on the vine.",
	"A fetched branch is a happy branch.",
	"The river flows from remote to local.",
	"Steady drizzle: one fetch at a time.",
}

func RandomStartupRainQuote() string {
	return PickRandomString(startupRainQuotes)
}

func StartupRainQuotes() []string {
	return append([]string(nil), startupRainQuotes...)
}
