// SPDX-License-Identifier: MIT

package gitignore

import (
	"bytes"
	"fmt"
	"math/rand"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/boyter/gocodewalker/go-gitignore/internal/fnmatch"
)

// ---------------------------------------------------------------------------
// Reference implementation
//
// This is name.Match as it was before the base name was hoisted out of the
// pattern loop into pathInfo: it derived the base name for itself, once per
// pattern per path. It is kept verbatim so the hoisted form can be
// differentially tested against the behaviour it is required to preserve.
// ---------------------------------------------------------------------------

// nameMatchReference is name.Match before the pathInfo hoist.
func nameMatchReference(n *name, path string, isdir bool) bool {
	// are we expecting a directory?
	if n._directory && !isdir {
		return false
	}

	// determine the string to match against
	_target := path
	if !n._anchored {
		i := strings.LastIndexByte(path, '/')
		if runtime.GOOS == "windows" {
			if j := strings.LastIndexByte(path, '\\'); j > i {
				i = j
			}
		}
		_target = path[i+1:]
	} else if strings.ContainsRune(_target, '/') {
		return false
	}

	switch n._matchType {
	case matchExact:
		return _target == n._literal
	case matchSuffix:
		return strings.HasSuffix(_target, n._literal)
	case matchPrefix:
		return strings.HasPrefix(_target, n._literal)
	case matchContains:
		return strings.Contains(_target, n._literal)
	case matchAny:
		return true
	default:
		return fnmatch.Match(n._fnmatch, _target, 0)
	}
}

// relativeReference is ignore.Relative before the pathInfo hoist: it derives
// nothing up front and hands each pattern the whole path.
func relativeReference(i *ignore, path string, isdir bool) Match {
	_rel := path
	if runtime.GOOS == "windows" {
		_rel = filepath.ToSlash(_rel)
	}

	for _i := len(i._pattern) - 1; _i >= 0; _i-- {
		_pattern := i._pattern[_i]

		var _matched bool
		if _name, _ok := _pattern.(*name); _ok {
			_matched = nameMatchReference(_name, _rel, isdir)
		} else {
			_matched = _pattern.Match(_rel, isdir)
		}

		if _matched {
			return _pattern
		}
	}
	return nil
}

// diffPatterns covers every shape the hoist has to preserve: anchored and
// unanchored, directory only, negated, name, path and '**' patterns, and the
// escaped globs that must stay literal.
var diffPatterns = []string{
	// plain names, one per fast path bucket plus the complex fallback
	"node_modules", "*.o", ".*", "*test*", "*", "**", "*.[ch]", "foo?bar",
	`abc\*`, `\*`,
	// anchored names: the branch that reads path rather than base
	"/build", "/*", "/*.o", "/.*", "/node_modules",
	// directory only, in both anchored and unanchored form
	"build/", "/build/", "*.d/", ".*/",
	// negated, which must still come out of the same loop position
	"!keep", "!*.o", "!/build", "!build/",
	// path patterns, which are matched whole
	"a/b", "src/*.o", "a/b/c", "/a/b",
	// '**' in each position
	"**/foo", "a/**", "a/**/b", "**/a/**",
	// pathological but legal
	".", "..", "a", "a*b*c",
}

// diffPaths covers paths with and without separators, empty and dot base
// names, trailing slashes, deep nesting and non-ASCII.
var diffPaths = []string{
	"", "/", "a", "a/", "/a", "a/b", "a/b/", "a/b/c", "a/b/c/d/e",
	"node_modules", "a/node_modules", "node_modules/x", "build", "build/x",
	"foo.o", "a/foo.o", "a/b/foo.o", ".gitignore", "a/.gitignore",
	".", "..", "a/.", "a/..", "a//b", "//a", "keep", "a/keep", "keep/sub",
	"foo?bar", "abc*", "*", "x.c", "x.h", "src/x.o", "a/b/x.o",
	"ünïcødé.o", "a/ünïcødé/b.o", "a.d", "a.d/b", strings.Repeat("a/", 12) + "z.o",
}

// TestPathInfoHoistMatchesPerPatternDerivation is the differential test for the
// hoist. Relative now derives the base name once for the whole pattern list;
// the reference above derives it inside each pattern, as the code did before.
// The two must agree on which pattern wins for every (pattern, path, isdir)
// triple, not merely on whether anything matched, since the winning pattern is
// what decides ignore versus include.
func TestPathInfoHoistMatchesPerPatternDerivation(t *testing.T) {
	// each pattern on its own, so a disagreement names the pattern
	for _, _p := range diffPatterns {
		_ignore := New(bytes.NewBufferString(_p+"\n"), "/base", nil).(*ignore)
		comparePatternList(t, _ignore, _p)
	}

	// and the whole list at once, so ordering and negation are exercised
	_all := New(bytes.NewBufferString(strings.Join(diffPatterns, "\n")+"\n"), "/base", nil).(*ignore)
	comparePatternList(t, _all, "<all patterns>")
}

func comparePatternList(t *testing.T, i *ignore, label string) {
	t.Helper()

	for _, _path := range diffPaths {
		for _, _isdir := range []bool{false, true} {
			_want := relativeReference(i, _path, _isdir)
			_got := i.Relative(_path, _isdir)

			if matchIdentity(_want) != matchIdentity(_got) {
				t.Errorf(
					"%s: Relative(%q, %v) = %s, per-pattern derivation gives %s",
					label, _path, _isdir, matchIdentity(_got), matchIdentity(_want),
				)
			}
		}
	}
}

// matchIdentity renders a Match so that no match, a match on the empty pattern
// and a match on two patterns with the same text but different positions are
// all distinguishable.
func matchIdentity(m Match) string {
	if m == nil {
		return "<nil>"
	}
	return fmt.Sprintf("%q@%d:%d ignore=%v", m.String(), m.Position().Line, m.Position().Column, m.Ignore())
}

// TestPathInfoAgreesWithExportedMatch checks the other direction: the exported
// Pattern.Match, which is part of the public API and still takes a bare path,
// must give the same answer as the internal matchInfo it now delegates to.
func TestPathInfoAgreesWithExportedMatch(t *testing.T) {
	for _, _p := range diffPatterns {
		_ignore := New(bytes.NewBufferString(_p+"\n"), "/base", nil).(*ignore)
		if len(_ignore._pattern) == 0 {
			continue
		}
		_pattern := _ignore._pattern[0]

		for _, _path := range diffPaths {
			for _, _isdir := range []bool{false, true} {
				_want := _pattern.Match(_path, _isdir)
				_got := _pattern.matchInfo(newPathInfo(_path), _isdir)
				if _want != _got {
					t.Errorf(
						"pattern %q: Match(%q, %v) = %v but matchInfo = %v",
						_p, _path, _isdir, _want, _got,
					)
				}
			}
		}
	}
}

// TestNewPathInfo pins the derivation itself, including the cases the pattern
// corpus reaches only indirectly: an empty path, a path that is nothing but a
// separator, and a trailing separator leaving an empty base name.
func TestNewPathInfo(t *testing.T) {
	for _, _test := range []struct {
		Path   string
		Base   string
		HasSep bool
	}{
		{"", "", false},
		{"a", "a", false},
		{"/", "", true},
		{"/a", "a", true},
		{"a/", "", true},
		{"a/b", "b", true},
		{"a/b/c", "c", true},
		{"a//b", "b", true},
		{".gitignore", ".gitignore", false},
		{"a/.gitignore", ".gitignore", true},
		{"ünïcødé/b.o", "b.o", true},
	} {
		_info := newPathInfo(_test.Path)
		if _info.path != _test.Path || _info.base != _test.Base || _info.hasSep != _test.HasSep {
			t.Errorf(
				"newPathInfo(%q) = {path:%q base:%q hasSep:%v}; expected {base:%q hasSep:%v}",
				_test.Path, _info.path, _info.base, _info.hasSep, _test.Base, _test.HasSep,
			)
		}
	}
}

// TestPathInfoHoistRandomised is the same differential test over generated
// input rather than a hand-picked corpus, so that combinations nobody thought
// to write down are covered too. The generator is seeded, so any failure is
// reproducible.
func TestPathInfoHoistRandomised(t *testing.T) {
	_rand := rand.New(rand.NewSource(20240915))

	_fragments := []string{
		"a", "b", "foo", "bar", ".hidden", "x.o", "x.c", "node_modules",
		"*", "**", "*.o", ".*", "?", "[ab]", "*a*", `\*`, `\?`, "a*b",
		".", "..", "", "ünïcødé",
	}

	for _case := 0; _case < 5000; _case++ {
		// a pattern of one to three fragments, optionally negated, anchored
		// and directory only
		_parts := make([]string, 1+_rand.Intn(3))
		for _i := range _parts {
			_parts[_i] = _fragments[_rand.Intn(len(_fragments))]
		}
		_pattern := strings.Join(_parts, "/")
		if _rand.Intn(4) == 0 {
			_pattern = "/" + _pattern
		}
		if _rand.Intn(4) == 0 {
			_pattern += "/"
		}
		if _rand.Intn(4) == 0 {
			_pattern = "!" + _pattern
		}

		// a path of one to four fragments, optionally rooted or with a
		// trailing separator
		_segments := make([]string, 1+_rand.Intn(4))
		for _i := range _segments {
			_segments[_i] = _fragments[_rand.Intn(len(_fragments))]
		}
		_path := strings.Join(_segments, "/")
		if _rand.Intn(8) == 0 {
			_path = "/" + _path
		}
		if _rand.Intn(8) == 0 {
			_path += "/"
		}

		_ignore, _ok := New(bytes.NewBufferString(_pattern+"\n"), "/base", nil).(*ignore)
		if !_ok {
			continue
		}

		for _, _isdir := range []bool{false, true} {
			_want := relativeReference(_ignore, _path, _isdir)
			_got := _ignore.Relative(_path, _isdir)
			if matchIdentity(_want) != matchIdentity(_got) {
				t.Fatalf(
					"pattern %q path %q isdir %v: Relative = %s, per-pattern derivation gives %s",
					_pattern, _path, _isdir, matchIdentity(_got), matchIdentity(_want),
				)
			}
		}
	}
}
