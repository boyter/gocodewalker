package gitignore_test

import (
	"strings"
	"testing"
	"unicode/utf8"

	gitignore "github.com/boyter/gocodewalker/go-gitignore"

	"github.com/danwakefield/fnmatch"
)

// The fast paths that answer a name pattern without calling fnmatch compare
// bytes. fnmatch compares decoded runes, and decoding invalid UTF-8 yields
// U+FFFD for every bad byte -- on both sides. A pattern holding U+FFFD, which
// is what the lexer produces from invalid UTF-8 in a .gitignore file, therefore
// matches any invalid byte under fnmatch, and no byte comparison can reproduce
// that. These patterns have to go to fnmatch.
//
// Reaching this needs a .gitignore that is not valid UTF-8, which is rare, but
// the walker should not disagree with itself when it happens.
func TestNameFastPathInvalidUTF8(t *testing.T) {
	// \xd7 alone is not valid UTF-8, so the lexer reads it as U+FFFD, and \xff
	// in a filename decodes to U+FFFD too -- which is why fnmatch matches them
	_cases := []struct {
		Pattern string
		Name    string
	}{
		{"\xd7", "\xff"},
		{"*\xd7", "a\xff"},
		{"\xd7*", "\xffa"},
		{"*\xd7*", "a\xffb"},
		{"\xd7", "\xfe"},
		{"\xd7\xd7", "\xff\xff"},
		// the replacement character written out properly must behave the same
		// way, since by the time a pattern is compiled the two are one string
		{string(utf8.RuneError), "\xff"},
		// a valid pattern against an invalid name must still agree
		{"*.go", "\xff.go"},
		{"a*", "\xffa"},
	}

	for _, _case := range _cases {
		_ignore := gitignore.New(strings.NewReader(_case.Pattern+"\n"), "/base", nil)

		_fast := _ignore.Relative(_case.Name, false) != nil
		_slow := fnmatch.Match(_case.Pattern, _case.Name, 0)

		if _fast != _slow {
			t.Errorf(
				"pattern %q against name %q: fast path %v, fnmatch %v",
				_case.Pattern, _case.Name, _fast, _slow,
			)
		}
	}
}

// TestNameFastPathValidUTF8Unaffected makes sure the guard above is narrow: a
// pattern that holds no U+FFFD must still take the fast path it always did.
func TestNameFastPathValidUTF8Unaffected(t *testing.T) {
	_cases := []struct {
		Pattern string
		Name    string
		Match   bool
	}{
		{"node_modules", "node_modules", true},
		{"node_modules", "node_modulez", false},
		{"*.o", "kernel.o", true},
		{"*.o", "kernel.c", false},
		{".*", ".gitignore", true},
		{".*", "gitignore", false},
		{"*.o.*", "a.o.b", true},
		{"*", "anything", true},
		// non-ASCII but perfectly valid, so not the case the guard is for
		{"中文*", "中文.txt", true},
		{"*—dash", "em—dash", true},
	}

	for _, _case := range _cases {
		_ignore := gitignore.New(strings.NewReader(_case.Pattern+"\n"), "/base", nil)

		if _got := _ignore.Relative(_case.Name, false) != nil; _got != _case.Match {
			t.Errorf("pattern %q against name %q: got %v, want %v", _case.Pattern, _case.Name, _got, _case.Match)
		}
	}
}
