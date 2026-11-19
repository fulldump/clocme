package linguist

import (
	"bufio"
	"bytes"
	"path"
	"regexp"
	"strings"
)

// Attrs are the linguist attributes resolved for one path. Nil pointers mean
// "unspecified" (fall back to the default heuristics).
type Attrs struct {
	Vendored      *bool
	Generated     *bool
	Documentation *bool
	Language      string // linguist-language override, "" if none
}

type attrRule struct {
	re       *regexp.Regexp
	basename bool // pattern without slash: matched against the basename only
	attrs    map[string]*string
}

// Attributes is a parsed .gitattributes file (root only).
//
// Limitations compared with git:
//   - only the root .gitattributes is honoured (no nested files, no
//     info/attributes, no macros besides ignoring them);
//   - a pattern ending in "/" (which git never matches, since attributes do not
//     apply to directories) is leniently treated as "dir/**", because that is
//     what people usually mean with "vendor/ linguist-vendored";
//   - quoted patterns are not supported.
type Attributes struct {
	rules []attrRule
}

// ParseGitattributes parses the content of a .gitattributes file, keeping
// only rules that touch linguist-* attributes.
func ParseGitattributes(data []byte) *Attributes {
	a := &Attributes{}
	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "[attr]") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		attrs := map[string]*string{}
		for _, f := range fields[1:] {
			name, value := parseAttr(f)
			if !strings.HasPrefix(name, "linguist-") {
				continue
			}
			attrs[name] = value
		}
		if len(attrs) == 0 {
			continue
		}
		re, basename, ok := compilePattern(fields[0])
		if !ok {
			continue
		}
		a.rules = append(a.rules, attrRule{re: re, basename: basename, attrs: attrs})
	}
	return a
}

var (
	valTrue  = "true"
	valFalse = "false"
)

// parseAttr parses "attr", "-attr", "!attr" and "attr=value". A nil value
// means "unspecified" (the ! form).
func parseAttr(f string) (string, *string) {
	switch {
	case strings.HasPrefix(f, "-"):
		return f[1:], &valFalse
	case strings.HasPrefix(f, "!"):
		return f[1:], nil
	}
	if name, value, found := strings.Cut(f, "="); found {
		return name, &value
	}
	return f, &valTrue
}

// compilePattern converts a gitattributes glob into an anchored regexp.
func compilePattern(p string) (*regexp.Regexp, bool, bool) {
	if strings.HasPrefix(p, "!") { // negative patterns are forbidden in gitattributes
		return nil, false, false
	}
	if strings.HasSuffix(p, "/") {
		p += "**"
	}
	basename := !strings.Contains(p, "/")
	p = strings.TrimPrefix(p, "/")

	var sb strings.Builder
	sb.WriteString("^")
	for i := 0; i < len(p); i++ {
		c := p[i]
		switch {
		case c == '*' && strings.HasPrefix(p[i:], "**/"):
			sb.WriteString("(?:.*/)?")
			i += 2
		case c == '*' && strings.HasPrefix(p[i:], "**") && (i == 0 || p[i-1] == '/') && i+2 == len(p):
			sb.WriteString(".*")
			i++
		case c == '*':
			sb.WriteString("[^/]*")
		case c == '?':
			sb.WriteString("[^/]")
		case c == '[':
			end := strings.IndexByte(p[i+1:], ']')
			if end < 0 {
				sb.WriteString(`\[`)
				continue
			}
			class := p[i+1 : i+1+end]
			if strings.HasPrefix(class, "!") {
				class = "^" + class[1:]
			}
			sb.WriteString("[" + class + "]")
			i += end + 1
		case c == '\\' && i+1 < len(p):
			i++
			sb.WriteString(regexp.QuoteMeta(string(p[i])))
		default:
			sb.WriteString(regexp.QuoteMeta(string(c)))
		}
	}
	sb.WriteString("$")
	re, err := regexp.Compile(sb.String())
	if err != nil {
		return nil, false, false
	}
	return re, basename, true
}

// Lookup resolves the linguist attributes for a path. Later rules override
// earlier ones, attribute by attribute, as in git.
func (a *Attributes) Lookup(p string) Attrs {
	var out Attrs
	if a == nil {
		return out
	}
	base := path.Base(p)
	for _, r := range a.rules {
		subject := p
		if r.basename {
			subject = base
		}
		if !r.re.MatchString(subject) {
			continue
		}
		for name, value := range r.attrs {
			switch name {
			case "linguist-vendored":
				out.Vendored = boolAttr(value)
			case "linguist-generated":
				out.Generated = boolAttr(value)
			case "linguist-documentation":
				out.Documentation = boolAttr(value)
			case "linguist-language":
				if value == nil || *value == valTrue || *value == valFalse {
					out.Language = ""
				} else {
					out.Language = *value
				}
			}
		}
	}
	return out
}

func boolAttr(v *string) *bool {
	if v == nil {
		return nil
	}
	b := *v != valFalse
	return &b
}
