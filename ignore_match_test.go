// SPDX-License-Identifier: MIT

package gocodewalker

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/boyter/gocodewalker/go-gitignore"
)

// ignoreMatchForward is the scan ignoreMatch replaced, kept here as the
// reference the differential tests below compare against: every set walked in
// full, lowest priority first, each match overwriting the one before it. It
// lives in the test file rather than the package so that the implementation
// that was removed is not carried in the shipped code.
func ignoreMatchForward(matchPath string, isDir bool,
	globalIgnores, gitignores, ignores, customIgnores, moduleIgnores []gitignore.GitIgnore) (bool, SkipReason) {
	var shouldIgnore bool
	var skipReason SkipReason

	for _, sets := range []struct {
		ignores []gitignore.GitIgnore
		reason  SkipReason
	}{
		{globalIgnores, SkipReasonGlobalIgnore},
		{gitignores, SkipReasonGitignore},
		{ignores, SkipReasonIgnoreFile},
		{customIgnores, SkipReasonCustomIgnore},
		{moduleIgnores, SkipReasonModuleIgnore},
	} {
		for _, ignore := range sets.ignores {
			if m := ignore.MatchIsDir(matchPath, isDir); m != nil {
				shouldIgnore = m.Ignore()
				if shouldIgnore {
					skipReason = sets.reason
				} else {
					skipReason = ""
				}
			}
		}
	}

	return shouldIgnore, skipReason
} // ignoreMatchForward()

func mkIgnore(base string, lines ...string) gitignore.GitIgnore {
	return gitignore.New(strings.NewReader(strings.Join(lines, "\n")+"\n"), filepath.ToSlash(base), nil)
}

// TestIgnoreMatchForwardReverseEquivalent is the differential test for T2: the
// reverse early exit scan must return exactly the decision AND the SkipReason
// the forward full scan returns, for every set of ignore files and every path.
func TestIgnoreMatchForwardReverseEquivalent(t *testing.T) {
	base := "/repo"
	sub := "/repo/sub"

	type sets struct {
		name                                                 string
		globals, gitignores, ignores, customs, moduleIgnores []gitignore.GitIgnore
	}

	cases := []sets{
		{
			name:    "empty",
			globals: nil,
		},
		{
			name:       "global ignores, local un-ignores",
			globals:    []gitignore.GitIgnore{mkIgnore(base, "*.log")},
			gitignores: []gitignore.GitIgnore{mkIgnore(base, "!keep.log")},
		},
		{
			name:       "local re-ignores what global already ignored",
			globals:    []gitignore.GitIgnore{mkIgnore(base, "!*.log")},
			gitignores: []gitignore.GitIgnore{mkIgnore(base, "*.log")},
		},
		{
			name: "nested gitignores with negation at each level",
			gitignores: []gitignore.GitIgnore{
				mkIgnore(base, "*.o", "build/"),
				mkIgnore(sub, "!important.o", "*.tmp"),
			},
		},
		{
			name: "negation in the same file after the ignore",
			gitignores: []gitignore.GitIgnore{
				mkIgnore(base, "*.txt", "!notes.txt", "sub/notes.txt"),
			},
		},
		{
			name:       "directory only patterns",
			gitignores: []gitignore.GitIgnore{mkIgnore(base, "build/", "!build/keep/")},
		},
		{
			name:       "custom ignores override gitignores",
			gitignores: []gitignore.GitIgnore{mkIgnore(base, "*.go")},
			customs:    []gitignore.GitIgnore{mkIgnore(base, "!main.go")},
		},
		{
			name:    "ignore file and custom ignore disagree",
			ignores: []gitignore.GitIgnore{mkIgnore(base, "vendor/")},
			customs: []gitignore.GitIgnore{mkIgnore(base, "!vendor/")},
		},
		{
			name:          "module ignores last",
			gitignores:    []gitignore.GitIgnore{mkIgnore(base, "!sub")},
			moduleIgnores: []gitignore.GitIgnore{mkIgnore(base, "sub")},
		},
		{
			name:    "recursive globs and anchors",
			globals: []gitignore.GitIgnore{mkIgnore(base, "**/node_modules/", "/anchored.txt", "a/**/b")},
			gitignores: []gitignore.GitIgnore{
				mkIgnore(base, "!**/node_modules/keep"),
				mkIgnore(sub, "**", "!*.c"),
			},
		},
		{
			name: "all five populated",
			globals: []gitignore.GitIgnore{
				mkIgnore(base, "*.log", "*.o"),
				mkIgnore(base, "!debug.log"),
			},
			gitignores: []gitignore.GitIgnore{
				mkIgnore(base, "build/", "*.tmp"),
				mkIgnore(sub, "!*.tmp", "notes.txt"),
			},
			ignores: []gitignore.GitIgnore{mkIgnore(base, "*.bak", "!sub/x.bak")},
			customs: []gitignore.GitIgnore{mkIgnore(base, "secret*", "!secrets.md")},
			moduleIgnores: []gitignore.GitIgnore{
				mkIgnore(base, "sub/mod"),
			},
		},
	}

	paths := []string{
		"/repo/a.log", "/repo/keep.log", "/repo/debug.log", "/repo/main.go",
		"/repo/build", "/repo/build/keep", "/repo/build/keep/f.o", "/repo/anchored.txt",
		"/repo/vendor", "/repo/vendor/x.go", "/repo/node_modules", "/repo/node_modules/keep",
		"/repo/sub", "/repo/sub/notes.txt", "/repo/sub/important.o", "/repo/sub/x.tmp",
		"/repo/sub/x.bak", "/repo/sub/x.c", "/repo/sub/mod", "/repo/sub/mod/f.c",
		"/repo/a/deep/b", "/repo/secrets.md", "/repo/secret.key", "/repo/notes.txt",
		"/repo/sub/node_modules/pkg/index.js", "/outside/repo/a.log", "/repo",
	}

	for _, c := range cases {
		for _, p := range paths {
			for _, isDir := range []bool{false, true} {
				wantIgnore, wantReason := ignoreMatchForward(p, isDir, c.globals, c.gitignores, c.ignores, c.customs, c.moduleIgnores)
				gotIgnore, gotReason := ignoreMatch(p, isDir, c.globals, c.gitignores, c.ignores, c.customs, c.moduleIgnores)
				if wantIgnore != gotIgnore || wantReason != gotReason {
					t.Errorf("%s path=%s isDir=%v: forward=(%v,%q) reverse=(%v,%q)",
						c.name, p, isDir, wantIgnore, wantReason, gotIgnore, gotReason)
				}
			}
		}
	}
}

// TestIgnoreMatchForwardReverseFuzz throws randomly generated pattern sets at
// both implementations, since the hand written cases above can only cover the
// interactions that were thought of.
func TestIgnoreMatchForwardReverseFuzz(t *testing.T) {
	rnd := rand.New(rand.NewSource(1))
	atoms := []string{"*.log", "!*.log", "build/", "!build/", "a", "!a", "/a", "a/**", "**/a",
		"*.o", "!keep.o", "sub/*", "!sub/keep", "**", "!*.c", "x?.txt", "[ab].md"}
	names := []string{"a", "b", "keep.o", "x1.txt", "a.md", "build", "sub", "sub/keep",
		"sub/a.log", "a.log", "f.c", "build/x.o", "sub/deep/a"}

	randSet := func() []gitignore.GitIgnore {
		n := rnd.Intn(3)
		out := make([]gitignore.GitIgnore, 0, n)
		for i := 0; i < n; i++ {
			lines := make([]string, 0, 3)
			for j := 0; j < 1+rnd.Intn(3); j++ {
				lines = append(lines, atoms[rnd.Intn(len(atoms))])
			}
			bases := []string{"/repo", "/repo/sub"}
			out = append(out, mkIgnore(bases[rnd.Intn(len(bases))], lines...))
		}
		return out
	}

	for iter := 0; iter < 3000; iter++ {
		g, gi, ig, cu, mo := randSet(), randSet(), randSet(), randSet(), randSet()
		p := "/repo/" + names[rnd.Intn(len(names))]
		isDir := rnd.Intn(2) == 0
		wantIgnore, wantReason := ignoreMatchForward(p, isDir, g, gi, ig, cu, mo)
		gotIgnore, gotReason := ignoreMatch(p, isDir, g, gi, ig, cu, mo)
		if wantIgnore != gotIgnore || wantReason != gotReason {
			t.Fatalf("iter %d path=%s isDir=%v: forward=(%v,%q) reverse=(%v,%q)",
				iter, p, isDir, wantIgnore, wantReason, gotIgnore, gotReason)
		}
	}
}

// TestIgnoreMatchWalkGolden walks a tree of deliberately tricky ignore files
// and pins exactly which files come out and which are skipped, with the reason
// for each. The expectations below were taken from the walker as it behaved
// before the scan was rewritten, so this is what says the rewrite changed
// nothing that anyone can observe.
func TestIgnoreMatchWalkGolden(t *testing.T) {
	dir := t.TempDir()
	write := func(p, content string) {
		full := filepath.Join(dir, p)
		if err := os.MkdirAll(filepath.Dir(full), 0o777); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	write(".gitignore", "*.log\n*.o\nbuild/\n!keep.log\n")
	write(".ignore", "*.bak\n!important.bak\n")
	write("custom.ignore", "secret*\n!secrets.md\n")
	write("a.log", "x")
	write("keep.log", "x")
	write("a.o", "x")
	write("a.bak", "x")
	write("important.bak", "x")
	write("secret.key", "x")
	write("secrets.md", "x")
	write("main.go", "x")
	write("build/out.txt", "x")
	write("sub/.gitignore", "!*.o\n*.tmp\nnested/\n")
	write("sub/b.o", "x")
	write("sub/b.tmp", "x")
	write("sub/b.go", "x")
	write("sub/nested/deep.go", "x")
	write("sub/deeper/.gitignore", "!*.tmp\n")
	write("sub/deeper/c.tmp", "x")
	write("sub/deeper/c.go", "x")

	globalIgnore := filepath.Join(dir, "global.ignore")
	if err := os.WriteFile(globalIgnore, []byte("*.go\n!main.go\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	q := make(chan *File, 1000)
	w := NewFileWalker(dir, q)
	w.CustomIgnore = []string{"custom.ignore"}
	w.CustomIgnorePatterns = []string{"*.md", "!secrets.md"}
	w.CustomIgnoreFiles = []string{globalIgnore}

	var skips []string
	// root level subdirectories are walked concurrently, so the skip handler is
	// called from several goroutines at once
	var mu sync.Mutex
	w.SetSkipHandler(func(path, name string, isDir bool, reason SkipReason) {
		rel, _ := filepath.Rel(dir, path)
		mu.Lock()
		skips = append(skips, fmt.Sprintf("%s|%v|%s", filepath.ToSlash(rel), isDir, reason))
		mu.Unlock()
	})

	go func() { _ = w.Start() }()

	var files []string
	for f := range q {
		rel, _ := filepath.Rel(dir, f.Location)
		files = append(files, filepath.ToSlash(rel))
	}
	sort.Strings(files)
	sort.Strings(skips)

	wantFiles := []string{
		"custom.ignore",
		"global.ignore",
		"important.bak", // un-ignored by .ignore after *.bak
		"keep.log",      // un-ignored by .gitignore after *.log
		"main.go",       // un-ignored by the global ignore after *.go
		"secrets.md",    // un-ignored by a custom pattern after *.md
		"sub/b.o",       // un-ignored by a nested .gitignore overriding its parent
		"sub/deeper/c.tmp",
	}
	wantSkips := []string{
		".gitignore|false|hidden",
		".ignore|false|hidden",
		"a.bak|false|ignore_file",
		"a.log|false|gitignore",
		"a.o|false|gitignore",
		"build|true|gitignore",
		"secret.key|false|custom_ignore",
		"sub/.gitignore|false|hidden",
		"sub/b.go|false|global_ignore",
		"sub/b.tmp|false|gitignore",
		"sub/deeper/.gitignore|false|hidden",
		"sub/deeper/c.go|false|global_ignore",
		"sub/nested|true|gitignore",
	}

	if strings.Join(files, "\n") != strings.Join(wantFiles, "\n") {
		t.Errorf("emitted files differ\ngot:\n%s\nwant:\n%s",
			strings.Join(files, "\n"), strings.Join(wantFiles, "\n"))
	}
	if strings.Join(skips, "\n") != strings.Join(wantSkips, "\n") {
		t.Errorf("skip events differ\ngot:\n%s\nwant:\n%s",
			strings.Join(skips, "\n"), strings.Join(wantSkips, "\n"))
	}
}
