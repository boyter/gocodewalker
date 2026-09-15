// SPDX-License-Identifier: MIT

package gocodewalker

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/boyter/gocodewalker/go-gitignore"
)

func TestGitIgnore(t *testing.T) {
	testcases := []string{
		`/`,
		`\`,
		`"`,
		`.`,
	}

	abs, _ := filepath.Abs(".")
	for _, te := range testcases {
		gitignore.New(strings.NewReader(te), abs, nil)
	}
}

func FuzzTestGitIgnore(f *testing.F) {
	testcases := []string{
		"",
		`\`,
		`'`,
		`#`,
		"/",
		"README.md",
		`README.md
/`,
		`*.[oa]
*.html
*.min.js

!foo*.html
foo-excl.html

vmlinux*

\!important!.txt

log/*.log
!/log/foo.log

**/logdir/log
**/foodir/bar
exclude/**

!findthis*

**/hide/**
subdir/subdir2/

/rootsubdir/

dirpattern/

README.md
`,
	}

	for _, tc := range testcases {
		f.Add(tc) // Use f.Add to provide a seed corpus
	}

	abs, _ := filepath.Abs(".")
	f.Fuzz(func(t *testing.T, c string) {
		gitignore.New(strings.NewReader(c), abs, nil)
	})
}

// TestGitIgnoreMatchDoesNotPanic pins the patterns that used to crash the
// walker rather than answer. A .gitignore holding a character class that is
// never closed, with a member of more than one byte, ran off the end of the
// pattern inside fnmatch: "[中" is enough, and so is "[" followed by any byte
// that is not valid UTF-8, which the lexer turns into U+FFFD.
//
// Such a pattern matches nothing, which is what git does with an unterminated
// class -- `git check-ignore` against a .gitignore of "[a" ignores nothing at
// all, not even a file named "[a".
func TestGitIgnoreMatchDoesNotPanic(t *testing.T) {
	patterns := []string{
		"[\x95",       // invalid UTF-8, the form originally reported
		"**/[\x95",    // the same reached through a recursive pattern
		"[中",          // three byte rune, and perfectly valid UTF-8
		"[é",          // two byte rune
		"[\U0001F600", // four byte rune
		"[a-中",        // as the upper end of a range
		"[!中",         // negated
		"dir/[中",      // in a path pattern
		"[",           // the ASCII forms, which never crashed
		"[a",
	}

	abs, _ := filepath.Abs(".")
	names := []string{"0", "a", "[a", "中", "dir/x", "\xff"}

	for _, pattern := range patterns {
		ignore := gitignore.New(strings.NewReader(pattern+"\n"), abs, nil)
		for _, name := range names {
			for _, isDir := range []bool{false, true} {
				if m := ignore.Relative(name, isDir); m != nil {
					t.Errorf("pattern %q matched %q (isDir %v); an unterminated class matches nothing",
						pattern, name, isDir)
				}
			}
		}
	}
}

// FuzzGitIgnoreMatch fuzzes matching, not just parsing. FuzzTestGitIgnore above
// only builds a GitIgnore and never asks it about a path, which is how a
// pattern that parsed cleanly and then panicked on every path it was matched
// against went unnoticed.
func FuzzGitIgnoreMatch(f *testing.F) {
	for _, seed := range [][2]string{
		{"[\x95", "0"},
		{"[中", "a"},
		{"*.[oa]\n!keep.o", "keep.o"},
		{"**/logdir/log", "a/b/logdir/log"},
		{"exclude/**", "exclude/a/b"},
		{"!findthis*", "findthisfile"},
		{`\!important!.txt`, "!important!.txt"},
		{"dirpattern/", "dirpattern"},
	} {
		f.Add(seed[0], seed[1])
	}

	abs, _ := filepath.Abs(".")
	f.Fuzz(func(t *testing.T, body string, path string) {
		// the matcher is quadratic in the length of the subject for a pattern
		// with several "*" in it, which is tested where that bound is set
		// rather than here
		if len(body) > 256 || len(path) > 128 {
			t.Skip()
		}

		ignore := gitignore.New(strings.NewReader(body), abs, nil)
		if ignore == nil {
			t.Skip()
		}

		// must answer rather than panic, on every entry point that matches
		_ = ignore.Relative(path, false)
		_ = ignore.Relative(path, true)
		_ = ignore.Match(path)
		_ = ignore.Ignore(path)
		_ = ignore.Include(path)
	})
}
