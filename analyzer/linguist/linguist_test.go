package linguist

import "testing"

func TestDefaults(t *testing.T) {
	cases := []struct {
		path     string
		excluded bool
	}{
		{"main.go", false},
		{"cmd/server/main.go", false},
		{"node_modules/left-pad/index.js", true},
		{"web/node_modules/x.js", true},
		{"vendor/github.com/x/y.go", true},
		{"third_party/lib.c", true},
		{"static/jquery-3.6.0.js", true},
		{"static/app.min.js", true},
		{"static/bootstrap.css", true},
		{"types/index.d.ts", true},
		{"src/index.ts", false},
		{"dist/bundle.js", true},
		{"docs/guide.md", true},
		{"doc/x.go", true},
		{"README.md", true},
		{"pkg/README", true},
		{"LICENSE", true},
		{"CHANGELOG.md", true},
		{"examples/hello.go", true},
		{"pkg/examples/hello.go", false}, // linguist anchors examples/ at root
		{"package-lock.json", true},
		{"go.sum", true},
		{"api/v1/user.pb.go", true},
		{"proto/user_pb2.py", true},
		{"Form1.Designer.cs", true},
		{".github/workflows/ci.yml", true},
		{"gradlew", true},
		{"src/vendored.go", false},
	}
	for _, c := range cases {
		got, _ := Classify(c.path, nil)
		if got != c.excluded {
			t.Errorf("Classify(%q) excluded=%v, want %v", c.path, got, c.excluded)
		}
	}
}

func TestGitattributes(t *testing.T) {
	a := ParseGitattributes([]byte(`
# comment
*.txt text
lib/** linguist-vendored
assets/ linguist-vendored
*.gen.go linguist-generated=true
vendor/** -linguist-vendored
docs/** linguist-documentation=false
*.inc linguist-language=PHP
/root.js linguist-vendored
**/fixtures/** linguist-vendored
special.js linguist-vendored
special.js !linguist-vendored
`))

	cases := []struct {
		path     string
		excluded bool
		lang     string
	}{
		{"lib/foo.go", true, ""},
		{"lib/deep/foo.go", true, ""},
		{"src/lib/foo.go", false, ""},
		{"assets/a.js", true, ""},
		{"x/y.gen.go", true, ""},
		{"vendor/foo.go", false, ""}, // explicitly not vendored
		{"docs/guide.go", false, ""}, // explicitly not documentation
		{"a/b.inc", false, "PHP"},
		{"root.js", true, ""},
		{"sub/root.js", false, ""},
		{"test/fixtures/a.go", true, ""},
		{"fixtures/a.go", true, ""},
		{"special.js", false, ""}, // reset to unspecified -> defaults
		{"x.txt", false, ""},
	}
	for _, c := range cases {
		got, lang := Classify(c.path, a)
		if got != c.excluded || lang != c.lang {
			t.Errorf("Classify(%q) = (%v,%q), want (%v,%q)", c.path, got, lang, c.excluded, c.lang)
		}
	}
}

func TestPatternClasses(t *testing.T) {
	a := ParseGitattributes([]byte("gen[0-9].go linguist-generated\nfile?.c linguist-vendored\n"))
	for p, want := range map[string]bool{
		"gen1.go":   true,
		"a/gen2.go": true,
		"genx.go":   false,
		"file1.c":   true,
		"file12.c":  false,
	} {
		if got, _ := Classify(p, a); got != want {
			t.Errorf("%q: got %v want %v", p, got, want)
		}
	}
}
