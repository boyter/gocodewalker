// SPDX-License-Identifier: MIT

package gocodewalker

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// These tests check the walker against git itself rather than against an
// expectation written down by hand, because git is the only authority for what
// a .gitignore means. A fixture is laid out on disk, `git ls-files --others
// --exclude-standard` is asked which files survive the ignore rules, and the
// walker has to emit exactly that set.
//
// They are skipped when git is not installed, so the suite still runs without
// it. git is invoked with its system and global configuration disabled, so that
// whatever the person running the tests has in ~/.gitconfig -- core.excludesFile
// especially, which would make git ignore files the walker knows nothing about
// -- cannot change the answer.

// gitEnv is the environment git is run with: no system config, no global
// config, and a HOME that holds neither.
func gitEnv(home string) []string {
	return append(os.Environ(),
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_CONFIG_GLOBAL=/dev/null",
		"GIT_CONFIG_SYSTEM=/dev/null",
		"HOME="+home,
		"XDG_CONFIG_HOME="+filepath.Join(home, ".config"),
		"GIT_TERMINAL_PROMPT=0",
	)
}

func runGit(t *testing.T, dir, home string, args ...string) string {
	t.Helper()

	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = gitEnv(home)

	out, err := cmd.Output()
	if err != nil {
		var stderr string
		if exit, ok := err.(*exec.ExitError); ok {
			stderr = string(exit.Stderr)
		}
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, stderr)
	}

	return string(out)
}

// hidden reports whether any component of a slash separated path begins with a
// dot. The walker leaves those out unless asked otherwise, while git has no
// such rule, so they are dropped from both sides rather than compared.
func hidden(path string) bool {
	for _, _part := range strings.Split(path, "/") {
		if strings.HasPrefix(_part, ".") {
			return true
		}
	}

	return false
}

// gitSurvivors is the set of files in dir that git does not ignore.
func gitSurvivors(t *testing.T, dir, home string) []string {
	t.Helper()

	var out []string
	for _, _line := range strings.Split(runGit(t, dir, home, "ls-files", "--others", "--exclude-standard"), "\n") {
		_line = strings.TrimSpace(_line)
		if _line == "" || hidden(_line) {
			continue
		}
		out = append(out, _line)
	}
	sort.Strings(out)

	return out
}

// walkerSurvivors is the set of files the walker emits for dir.
func walkerSurvivors(t *testing.T, dir string) []string {
	t.Helper()

	_queue := make(chan *File, 4096)
	_walker := NewFileWalker(dir, _queue)
	_walker.SetErrorHandler(func(e error) bool {
		t.Errorf("walk error: %v", e)

		return true
	})

	go func() { _ = _walker.Start() }()

	var out []string
	for _file := range _queue {
		_rel, err := filepath.Rel(dir, _file.Location)
		if err != nil {
			t.Fatal(err)
		}
		_rel = filepath.ToSlash(_rel)
		if hidden(_rel) {
			continue
		}
		out = append(out, _rel)
	}
	sort.Strings(out)

	return out
}

// gitConformanceCase is a tree plus the ignore rules covering it.
type gitConformanceCase struct {
	Name string

	// GitIgnore is keyed by the directory the file belongs in, "" for the root
	GitIgnore map[string]string

	// Exclude is the contents of .git/info/exclude
	Exclude string

	// Files are created empty, with their parent directories
	Files []string
}

func gitConformanceCases() []gitConformanceCase {
	return []gitConformanceCase{
		{
			// the case this test was written for: .gitignore outranks
			// info/exclude, in both directions
			Name:      "info/exclude ranks below .gitignore",
			GitIgnore: map[string]string{"": "*.o\n*.log\nbuild/\n!keep.log\nsecret*\n!secrets.md\n"},
			Exclude:   "keep.log\n!*.o\n*.bak\n!secret.key\nnotes.md\n",
			Files: []string{
				"a.o", "b.o", "a.log", "keep.log", "secret.key", "secrets.md",
				"notes.md", "main.go", "a.bak", "build/out.txt", "sub/c.o", "sub/d.log",
			},
		},
		{
			// a nested .gitignore overriding the one above it
			Name: "nested gitignore overrides its parent",
			GitIgnore: map[string]string{
				"":           "*.o\n*.tmp\n",
				"sub":        "!*.o\nnested/\n",
				"sub/deeper": "!*.tmp\n",
			},
			Files: []string{
				"a.o", "a.tmp", "main.go",
				"sub/b.o", "sub/b.tmp", "sub/b.go",
				"sub/nested/deep.go",
				"sub/deeper/c.o", "sub/deeper/c.tmp", "sub/deeper/c.go",
			},
		},
		{
			// anchoring: a leading slash means the root only
			Name:      "anchored patterns",
			GitIgnore: map[string]string{"": "/build\n/a.txt\nlogs\n"},
			Files: []string{
				"build/x.o", "a.txt", "logs/x.log",
				"sub/build/y.o", "sub/a.txt", "sub/logs/y.log", "sub/keep.go",
			},
		},
		{
			// directory-only patterns, which must not match a file of that name
			Name:      "directory only patterns",
			GitIgnore: map[string]string{"": "build/\ndist/\n"},
			Files: []string{
				"build/x.o", "dist", "keep.go", "sub/build/y.o", "sub/dist/z.o",
			},
		},
		{
			// recursive globs
			Name:      "double star patterns",
			GitIgnore: map[string]string{"": "**/logs\n**/gen/**\nvendor/**\na/**/b\n"},
			Files: []string{
				"logs/x.txt", "sub/logs/y.txt", "gen/a.go", "sub/gen/b.go",
				"vendor/pkg/c.go", "a/x/y/b/d.go", "a/b/e.go", "keep.go",
			},
		},
		{
			// character classes and escaped globs, where the fast paths in this
			// library either apply or deliberately fall back to fnmatch
			Name:      "classes and escapes",
			GitIgnore: map[string]string{"": "*.[oa]\nfoo?bar\n[abc]init\n*.min.*\n"},
			Files: []string{
				"x.o", "x.a", "x.c", "fooXbar", "foobar", "ainit", "dinit",
				"app.min.js", "app.js", "keep.go",
			},
		},
		{
			// negation restoring a file inside an ignored directory, which git
			// does not do: once a directory is excluded it is not descended into
			Name:      "negation cannot re-include inside an ignored directory",
			GitIgnore: map[string]string{"": "build/\n!build/keep.txt\n"},
			Files:     []string{"build/keep.txt", "build/drop.o", "keep.go"},
		},
		{
			// trailing "**" must not match the directory itself, the case fixed
			// in c231e40. The negation is what makes this discriminating: "abc/**"
			// excludes the contents but not abc itself, so abc is still descended
			// into and "!abc/keep.txt" can bring a file back. Without the
			// negation the directory being wrongly excluded looks identical from
			// the outside, since everything in it was ignored either way.
			Name:      "trailing double star excludes contents, not the directory",
			GitIgnore: map[string]string{"": "abc/**\n!abc/keep.txt\n"},
			Files:     []string{"abc/x.txt", "abc/keep.txt", "abc/d/y.txt", "abcd/z.txt", "keep.go"},
		},
		{
			// the contrast: a trailing slash does exclude the directory itself,
			// and then nothing inside it can be recovered
			Name:      "trailing slash excludes the directory itself",
			GitIgnore: map[string]string{"": "abc/\n!abc/keep.txt\n"},
			Files:     []string{"abc/x.txt", "abc/keep.txt", "abc/d/y.txt", "abcd/z.txt", "keep.go"},
		},
		{
			// whitespace, comments and escaped specials
			Name:      "comments and escapes",
			GitIgnore: map[string]string{"": "# a comment\n\n\\#hash.txt\n\\!bang.txt\ntrailing \n"},
			Files:     []string{"#hash.txt", "!bang.txt", "trailing", "keep.go"},
		},
	}
}

// TestGitConformance walks each fixture and requires the walker to emit exactly
// the files git says survive its ignore rules.
func TestGitConformance(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}

	for _, tc := range gitConformanceCases() {
		t.Run(tc.Name, func(t *testing.T) {
			// a HOME of its own, so that git reads no configuration but the
			// repository's, and the walker sees no stray files
			home := t.TempDir()
			dir := t.TempDir()

			runGit(t, dir, home, "init", "-q", ".")

			for _, name := range tc.Files {
				full := filepath.Join(dir, filepath.FromSlash(name))
				if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(full, []byte("x\n"), 0600); err != nil {
					t.Fatal(err)
				}
			}

			for where, body := range tc.GitIgnore {
				full := filepath.Join(dir, filepath.FromSlash(where), GitIgnore)
				if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(full, []byte(body), 0600); err != nil {
					t.Fatal(err)
				}
			}

			if tc.Exclude != "" {
				full := filepath.Join(dir, ".git", "info", "exclude")
				if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(full, []byte(tc.Exclude), 0600); err != nil {
					t.Fatal(err)
				}
			}

			want := gitSurvivors(t, dir, home)
			got := walkerSurvivors(t, dir)

			if strings.Join(got, "\n") != strings.Join(want, "\n") {
				t.Errorf("walker and git disagree\n git emits:\n   %s\n walker emits:\n   %s\n only git:    %v\n only walker: %v",
					strings.Join(want, "\n   "), strings.Join(got, "\n   "),
					missing(want, got), missing(got, want))
			}
		})
	}
}

// missing returns the members of a that are not in b.
func missing(a, b []string) []string {
	_in := make(map[string]struct{}, len(b))
	for _, _s := range b {
		_in[_s] = struct{}{}
	}

	var out []string
	for _, _s := range a {
		if _, _ok := _in[_s]; !_ok {
			out = append(out, _s)
		}
	}

	return out
}
