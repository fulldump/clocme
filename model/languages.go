package model

// DataLanguages are languages (by scc name) that GitHub Linguist classifies as
// "data" or "prose". They are always stored in the series but hidden from
// charts by default, because a single lockfile-like JSON or a big README can
// dwarf the actual code. They can be brought back with ?include=.
var DataLanguages = map[string]bool{
	"JSON":                  true,
	"JSONL":                 true,
	"JSONC":                 true,
	"YAML":                  true,
	"XML":                   true,
	"CSV":                   true,
	"TSV":                   true,
	"SVG":                   true,
	"TOML":                  true,
	"INI":                   true,
	"Markdown":              true,
	"Plain Text":            true,
	"License":               true,
	"ReStructuredText":      true,
	"AsciiDoc":              true,
	"Org":                   true,
	"Properties File":       true,
	"XML Schema":            true,
	"JSON5":                 true,
	"Patch":                 true,
	"gitignore":             true,
	"Docker ignore":         true,
	"ignore":                true,
	"Rich Text Format":      true,
	"SPDX":                  true,
	"CloudFormation (JSON)": true,
	"CloudFormation (YAML)": true,
}
