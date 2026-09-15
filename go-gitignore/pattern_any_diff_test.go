// SPDX-License-Identifier: MIT

package gitignore

import (
	"strings"
	"testing"

	"github.com/danwakefield/fnmatch"
)

// anyPatterns and anyPaths are the corpus for the differential test below. The
// patterns cover every shape of '**' pattern: leading, trailing, embedded,
// repeated, adjacent to globs, directory only, anchored and negated. The paths
// cover the awkward edges: the empty path, paths with empty components,
// trailing separators and paths that only partially match.
var anyPatterns = []string{
	"**", "/**", "**/", "**/**",
	"**/foo", "foo/**", "a/**/b", "a/**/**/b",
	"**/foo/**", "**/*.o", "**/build/**", "a/**",
	"**/node_modules", "node_modules/**", "**/*", "*/**",
	"**/a*", "**/*a", "**/*a*", "**/a?c", "**/[ab]c", `**/a\*`,
	"!**/foo", "**/foo/", "/a/**/b", "src/**/*.go",
	"**/**/**", "a/**/b/**/c", "**b", "b**", "a**b",
	"**/.*", "**/*.[ch]", "x/**/y/**", "**/x/y",
}

var anyPaths = []string{
	"", "a", "/", "a/", "/a", "a/b", "a/b/c", "foo", "a/foo", "a/b/foo",
	"foo/bar", "foo/a/b", "build", "build/x.o", "build/a/b/x.o",
	"a/x/b", "a/x/y/b", "a//b", "/a/b", "node_modules",
	"a/node_modules", "node_modules/x", "a/node_modules/b/c",
	"src/a/b.go", "src/b.go", "x.o", "a/x.o", ".git", "a/.git",
	"ac", "abc", "axc", "a*", "a/b/", "x/p/y/q", "x/y", "a/b/c/d/e",
	"bb", "ab", "aXb", "b", "x", "//", "///", "a//", "//a",
}

// TestAnyMatchOffsetAgreesWithSplit is a differential test between the previous
// 'any' matcher, which split the path into a []string, and the offset walking
// replacement. The two must be indistinguishable for every (pattern, path,
// isdir) triple: the offset encoding of "no components left" is the part most
// likely to diverge, and the empty path and paths with empty components are
// exactly where strings.Split behaves in a way an offset scan easily gets
// wrong.
func TestAnyMatchOffsetAgreesWithSplit(t *testing.T) {
	checked := 0
	for _, p := range anyPatterns {
		_pattern := parseOnePattern(t, p)
		if _pattern == nil {
			continue
		}
		_any, ok := _pattern.(*any)
		if !ok {
			t.Fatalf("pattern %q did not compile to an 'any' pattern, got %T", p, _pattern)
		}

		for _, path := range anyPaths {
			for _, isdir := range []bool{false, true} {
				want := _any.matchLegacy(path, isdir)
				got := _any.Match(path, isdir)
				if got != want {
					t.Errorf("pattern %q path %q isdir=%v: new matcher says %v, old says %v",
						p, path, isdir, got, want)
				}
				checked++
			}
		}
	}

	if checked == 0 {
		t.Fatal("differential test checked nothing")
	}
	t.Logf("checked %d (pattern, path, isdir) triples", checked)
}

// TestAnyMatchOffsetAgreesWithSplitGenerated widens the differential test to a
// generated cross product of pattern and path shapes, so that combinations the
// hand written corpus above did not think of are covered as well.
func TestAnyMatchOffsetAgreesWithSplitGenerated(t *testing.T) {
	segments := []string{"a", "b", "*", "**", "?", "*c", "c*"}

	// every pattern of one, two and three segments over the alphabet above
	var patterns []string
	for _, x := range segments {
		patterns = append(patterns, x)
		for _, y := range segments {
			patterns = append(patterns, x+"/"+y)
			for _, z := range segments {
				patterns = append(patterns, x+"/"+y+"/"+z)
			}
		}
	}

	pathSegments := []string{"a", "b", "c", "ac", "ca", ""}
	var paths []string
	for _, x := range pathSegments {
		paths = append(paths, x)
		for _, y := range pathSegments {
			paths = append(paths, x+"/"+y)
			for _, z := range pathSegments {
				paths = append(paths, x+"/"+y+"/"+z)
			}
		}
	}

	checked := 0
	for _, p := range patterns {
		if !strings.Contains(p, "**") {
			continue
		}
		_pattern := parseOnePattern(t, p)
		if _pattern == nil {
			continue
		}
		_any, ok := _pattern.(*any)
		if !ok {
			continue
		}

		for _, path := range paths {
			for _, isdir := range []bool{false, true} {
				want := _any.matchLegacy(path, isdir)
				got := _any.Match(path, isdir)
				if got != want {
					t.Errorf("pattern %q path %q isdir=%v: new matcher says %v, old says %v",
						p, path, isdir, got, want)
				}
				checked++
			}
		}
	}

	if checked == 0 {
		t.Fatal("differential test checked nothing")
	}
	t.Logf("checked %d (pattern, path, isdir) triples", checked)
}

// FuzzAnyMatchAgainstSplit widens the differential tests above to arbitrary
// input: the fuzzer picks the pattern as well as the path, so pattern shapes
// the corpora did not anticipate are covered too. Only patterns that compile to
// an 'any' pattern are of interest; everything else is skipped.
func FuzzAnyMatchAgainstSplit(f *testing.F) {
	for i, p := range anyPatterns {
		f.Add(p, anyPaths[i%len(anyPaths)])
	}
	for _, p := range anyPaths {
		f.Add("**/"+p, p)
	}

	f.Fuzz(func(t *testing.T, line, path string) {
		// a pattern is a single line, and NUL bytes are not gitignore input
		if strings.ContainsAny(line, "\n\r\x00") || strings.ContainsAny(path, "\x00") {
			t.Skip()
		}

		// The recursion behind '**' is exponential in the number of '**'
		// tokens for a long enough path, in this implementation and in the one
		// it replaces alike. That is worth knowing but it is not what this test
		// is for, and letting the fuzzer find it just exhausts the worker, so
		// keep the inputs to the size a real pattern and path have.
		if len(line) > 64 || len(path) > 128 {
			t.Skip()
		}

		_patterns := NewParser(strings.NewReader(line+"\n"), nil).Parse()
		if len(_patterns) == 0 {
			t.Skip()
		}
		_any, ok := _patterns[len(_patterns)-1].(*any)
		if !ok {
			t.Skip()
		}

		for _, isdir := range []bool{false, true} {
			// the vendored fnmatch panics on some malformed patterns (an
			// unterminated character class containing invalid UTF-8, for
			// instance), which it did before this change too, so the two
			// implementations only have to agree on panicking, not avoid it
			want, wantPanic := recovered(func() bool { return _any.matchLegacy(path, isdir) })
			got, gotPanic := recovered(func() bool { return _any.Match(path, isdir) })

			if wantPanic != gotPanic {
				t.Fatalf("pattern %q path %q isdir=%v: new matcher panicked=%v, old panicked=%v",
					line, path, isdir, gotPanic, wantPanic)
			}
			if !wantPanic && got != want {
				t.Fatalf("pattern %q path %q isdir=%v: new matcher says %v, old says %v",
					line, path, isdir, got, want)
			}
		}
	})
}

// recovered runs f, reporting its result and whether it panicked.
func recovered(f func() bool) (result bool, panicked bool) {
	defer func() {
		if recover() != nil {
			panicked = true
		}
	}()
	return f(), false
}

// matchLegacy is the 'any' matcher as it was before the offset walking change,
// retained here so the differential tests above have something to compare
// against. It lives in the test file rather than in pattern.go so that the
// implementation it replaced is not carried in the shipped package.
func (a *any) matchLegacy(path string, isdir bool) bool {
	if a._directory && !isdir {
		return false
	}
	return matchLegacyTokens(strings.Split(path, string(_SEPARATOR)), a._tokens)
}

// matchLegacyTokens is the recursive half of the previous 'any' matcher, copied
// verbatim from pattern.go before the change. An 'any' token '**' may match any
// path component, or no path component.
func matchLegacyTokens(path []string, tokens []*Token) bool {
	// if we have no more tokens, then we have matched this path
	// if there are also no more path elements, otherwise there's no match
	if len(tokens) == 0 {
		return len(path) == 0
	}

	// what token are we trying to match?
	_token := tokens[0]
	switch _token.Type {
	case ANY:
		if len(tokens) == 1 {
			return len(path) != 0
		}
		if len(path) == 0 {
			return matchLegacyTokens(path, tokens[1:])
		}
		return matchLegacyTokens(path, tokens[1:]) || matchLegacyTokens(path[1:], tokens)

	default:
		// if we have a non-ANY token, then we must have a non-empty path
		if len(path) != 0 {
			// if the current path element matches this token, we match if the
			// remainder of the path matches the remaining tokens
			if fnmatch.Match(_token.Token(), path[0], fnmatch.FNM_PATHNAME) {
				return matchLegacyTokens(path[1:], tokens[1:])
			}
		}
	}

	// if we are here, then we have no match
	return false
}

// parseOnePattern parses a single gitignore line and returns the compiled
// Pattern, or nil if the line yielded none.
func parseOnePattern(t *testing.T, line string) Pattern {
	t.Helper()

	_patterns := NewParser(strings.NewReader(line+"\n"), nil).Parse()
	if len(_patterns) == 0 {
		return nil
	}
	return _patterns[len(_patterns)-1]
}
