// Package linguist ports a subset of GitHub Linguist's path heuristics
// (vendor.yml, documentation.yml and the path based rules of generated.rb) plus
// the linguist-* attributes of a root .gitattributes file.
//
// Linguist rules are regular expressions over the slash separated path
// relative to the repository root. They are reproduced here as Go regexps;
// only the path based ones are ported (content based generated detection is
// delegated to scc's "generated" markers and minified heuristics).
package linguist

import (
	"regexp"
	"strings"
)

// vendorPatterns is a subset of
// https://github.com/github-linguist/linguist/blob/main/lib/linguist/vendor.yml
var vendorPatterns = []string{
	// Caches, dependency and build folders
	`(^|/)cache/`,
	`^[Dd]ependencies/`,
	`(^|/)dist/`,
	`^deps/`,
	`(^|/)node_modules/`,
	`(^|/)\.yarn/(releases|plugins|sdks|versions|unplugged)/`,
	`(^|/)bower_components/`,
	`(^|/)jspm_packages/`,
	`(^|/)Godeps/_workspace/`,
	`(^|/)_esy$`,
	`^rebar$`,
	`(^|/)erlang\.mk`,
	`(3rd|[Tt]hird)[-_]?[Pp]arty/`,
	`(^|/)vendors?/`,
	`(^|/)[Vv]+endor/`,
	`(^|/)[Ee]xtern(als?)?/`,
	`^debian/`,
	`(^|/)Pods/`,
	`(^|/)Carthage/`,
	`(^|/)Sparkle/`,
	`(^|/)[^/]+\.framework/`,
	`\.xctemplate/`,
	`\.imageset/`,
	`(^|/)[Pp]ackages/.+\.\d+/`,
	`(^|/)\.venv/`,
	`(^|/)venv/`,
	`(^|/)env/`,
	`(^|/)site-packages/`,

	// Autotools / build helpers
	`(^|/)configure$`,
	`(^|/)config\.guess$`,
	`(^|/)config\.sub$`,
	`(^|/)aclocal\.m4`,
	`(^|/)libtool\.m4`,
	`(^|/)ltoptions\.m4`,
	`(^|/)ltsugar\.m4`,
	`(^|/)ltversion\.m4`,
	`(^|/)lt~obsolete\.m4`,
	`(^|/)gradlew(\.bat)?$`,
	`(^|/)gradle/wrapper/`,
	`(^|/)mvnw(\.cmd)?$`,
	`(^|/)\.mvn/wrapper/`,
	`(^|/)waf$`,
	`(^|/)fabfile\.py$`,

	// Editor / CI / repo config
	`(^|/)\.vscode/`,
	`(^|/)\.idea/`,
	`(^|/)\.github/`,
	`(^|/)\.sublime-(project|workspace)`,
	`(^|/)\.git(attributes|ignore|modules)$`,

	// Minified files and well known third party libraries
	`(\.|-)min\.(js|css)$`,
	`([^\s]*)import\.(css|less|scss|styl)$`,
	`(^|/)bootstrap([^/.]*)(\..*)?\.(js|css|less|scss|styl)$`,
	`(^|/)custom\.bootstrap([^\s]*)(js|css|less|scss|styl)$`,
	`(^|/)font-?awesome\.(css|less|scss|styl)$`,
	`(^|/)font-?awesome/.*\.(css|less|scss|styl)$`,
	`(^|/)fontello(.*?)\.(css|js)$`,
	`(^|/)animate\.(css|less|scss|styl)$`,
	`(^|/)materialize\.(css|less|scss|styl|js)$`,
	`(^|/)select2/.*\.(css|scss|js)$`,
	`(^|/)bulma\.(css|sass|scss)$`,
	`(^|/)jquery([^.]*)\.js$`,
	`(^|/)jquery-\d\.\d+(\.\d+)?\.js$`,
	`(^|/)jquery-ui(-\d\.\d+(\.\d+)?)?(\.\w+)?\.(js|css)$`,
	`(^|/)jquery\.(ui|effects)\.([^.]*)\.(js|css)$`,
	`(^|/)prototype(.*)\.js$`,
	`(^|/)mootools([^.]*)\d+\.\d+.\d+([^.]*)\.js$`,
	`(^|/)dojo\.js$`,
	`(^|/)MochiKit\.js$`,
	`(^|/)yahoo-([^.]*)\.js$`,
	`(^|/)yui([^.]*)\.js$`,
	`(^|/)ckeditor\.js$`,
	`(^|/)tiny_mce([^.]*)\.js$`,
	`(^|/)ace-builds/`,
	`(^|/)MathJax/`,
	`(^|/)Chart\.js$`,
	`(^|/)[Cc]ode[Mm]irror/(\d+\.\d+/)?(lib|mode|theme|addon|keymap|demo)`,
	`(^|/)angular([^.]*)\.js$`,
	`(^|/)d3(\.v\d+)?([^.]*)\.js$`,
	`(^|/)react(-[^.]*)?\.js$`,
	`(^|/)flow-typed/.*\.js$`,
	`(^|/)modernizr-\d\.\d+(\.\d+)?\.js$`,
	`(^|/)modernizr\.custom\.\d+\.js$`,
	`(^|/)knockout-(\d+\.){3}(debug\.)?js$`,
	`(^|/)pdf\.worker\.js$`,
	`-vsdoc\.js$`,
	`\.intellisense\.js$`,
	`(^|/)extjs/.*?\.js$`,
	`(.*?)\.d\.ts$`,
	`(^|/)docs?/_?(build|themes?|templates?|static)/`,
}

// documentationPatterns is
// https://github.com/github-linguist/linguist/blob/main/lib/linguist/documentation.yml
var documentationPatterns = []string{
	`^[Dd]ocs?/`,
	`(^|/)[Dd]ocumentation/`,
	`(^|/)[Gg]roovydoc/`,
	`(^|/)[Jj]avadoc/`,
	`^[Mm]an/`,
	`^[Ee]xamples/`,
	`^[Dd]emos?/`,
	`(^|/)inst/doc/`,
	`(^|/)CITATION(\.cff|(S)?(\.(bib|md))?)$`,
	`(^|/)CHANGE(S|LOG)?(\.|$)`,
	`(^|/)CONTRIBUTING(\.|$)`,
	`(^|/)COPYING(\.|$)`,
	`(^|/)INSTALL(\.|$)`,
	`(^|/)LICEN[CS]E(\.|$)`,
	`(^|/)[Ll]icen[cs]e(\.|$)`,
	`(^|/)README(\.|$)`,
	`(^|/)[Rr]eadme(\.|$)`,
	`^[Ss]amples?/`,
}

// generatedPatterns are the path based rules of
// https://github.com/github-linguist/linguist/blob/main/lib/linguist/generated.rb
// (lockfiles, protobuf/grpc output, designer files, source maps...).
var generatedPatterns = []string{
	// Lockfiles
	`(^|/)package-lock\.json$`,
	`(^|/)npm-shrinkwrap\.json$`,
	`(^|/)yarn\.lock$`,
	`(^|/)pnpm-lock\.yaml$`,
	`(^|/)bun\.lockb?$`,
	`(^|/)deno\.lock$`,
	`(^|/)composer\.lock$`,
	`(^|/)Gemfile\.lock$`,
	`(^|/)go\.sum$`,
	`(^|/)go\.work\.sum$`,
	`(^|/)Gopkg\.lock$`,
	`(^|/)glide\.lock$`,
	`(^|/)Cargo\.lock$`,
	`(^|/)poetry\.lock$`,
	`(^|/)Pipfile\.lock$`,
	`(^|/)pdm\.lock$`,
	`(^|/)uv\.lock$`,
	`(^|/)flake\.lock$`,
	`(^|/)mix\.lock$`,
	`(^|/)pubspec\.lock$`,
	`(^|/)Podfile\.lock$`,
	`(^|/)Package\.resolved$`,
	`(^|/)packages\.lock\.json$`,
	`(^|/)paket\.lock$`,
	`(^|/)\.pnp\.c?js$`,
	`(^|/)\.pnp\.loader\.mjs$`,

	// Protobuf / gRPC / Thrift / FlatBuffers
	`\.pb\.(go|cc|h|swift|c)$`,
	`\.pb\.gw\.go$`,
	`_grpc\.pb\.go$`,
	`_pb2(_grpc)?\.pyi?$`,
	`_pb\.(js|d\.ts|rb)$`,
	`_grpc_pb\.(js|d\.ts)$`,
	`(^|/)gen-(cpp|java|py|rb|go|js)/`,
	`_generated\.(h|go|rs|ts)$`,

	// .NET / Dart / misc code generators
	`\.[Dd]esigner\.(cs|vb)$`,
	`\.g\.(cs|dart|i\.cs)$`,
	`\.freezed\.dart$`,
	`\.mocks\.dart$`,
	`(^|/)__generated__/`,
	`(^|/)generated/`,
	`\.(js|css)\.map$`,
	`\.map$`,
	`(^|/)project\.pbxproj$`,
	`\.xcworkspacedata$`,
	`\.xcuserstate$`,
	`(^|/)\.terraform\.lock\.hcl$`,
}

var (
	reVendor        = compileAll(vendorPatterns)
	reDocumentation = compileAll(documentationPatterns)
	reGenerated     = compileAll(generatedPatterns)
)

func compileAll(patterns []string) *regexp.Regexp {
	return regexp.MustCompile("(?:" + strings.Join(patterns, ")|(?:") + ")")
}

// IsVendored reports whether the path looks like third party code.
func IsVendored(p string) bool { return reVendor.MatchString(p) }

// IsDocumentation reports whether the path looks like documentation.
func IsDocumentation(p string) bool { return reDocumentation.MatchString(p) }

// IsGenerated reports whether the path looks like generated code (by name only).
func IsGenerated(p string) bool { return reGenerated.MatchString(p) }

// Classify decides whether a file at path p must be excluded from the stats
// and which language override (if any) applies, combining the default rules
// with the attributes from .gitattributes (attrs may be nil). Explicit
// attributes always win over the defaults, as in Linguist.
func Classify(p string, attrs *Attributes) (excluded bool, language string) {
	var a Attrs
	if attrs != nil {
		a = attrs.Lookup(p)
	}
	pick := func(explicit *bool, def func(string) bool) bool {
		if explicit != nil {
			return *explicit
		}
		return def(p)
	}
	excluded = pick(a.Vendored, IsVendored) ||
		pick(a.Generated, IsGenerated) ||
		pick(a.Documentation, IsDocumentation)
	return excluded, a.Language
}
