// SPDX-License-Identifier: MIT

package gitignore_test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/boyter/gocodewalker/go-gitignore"
)

// ---------------------------------------------------------------------------
// 1. Name-pattern fast-path tests (exact, suffix, complex)
// ---------------------------------------------------------------------------

// nameMatchTest describes a single name-pattern matching assertion.
type nameMatchTest struct {
	pattern string // gitignore pattern line
	path    string // relative path to test
	isdir   bool
	match   bool // true if the pattern should match
}

// runNameTests creates a GitIgnore from each pattern and checks Relative().
func runNameTests(t *testing.T, tests []nameMatchTest) {
	t.Helper()
	for _, tc := range tests {
		_ignore := gitignore.New(
			bytes.NewBufferString(tc.pattern+"\n"),
			"/base",
			nil,
		)
		_match := _ignore.Relative(tc.path, tc.isdir)
		got := _match != nil
		if got != tc.match {
			t.Errorf(
				"pattern %q, path %q, isdir=%v: expected match=%v, got %v",
				tc.pattern, tc.path, tc.isdir, tc.match, got,
			)
		}
	}
}

// TestNamePatternExactMatch exercises the matchExact fast-path:
// patterns with no glob characters (*, ?, [, \) use string ==.
func TestNamePatternExactMatch(t *testing.T) {
	runNameTests(t, []nameMatchTest{
		// basic exact matches
		{"node_modules", "node_modules", false, true},
		{"node_modules", "node_modules", true, true},
		{".DS_Store", ".DS_Store", false, true},
		{"Thumbs.db", "Thumbs.db", false, true},

		// non-anchored: match the last path component
		{"node_modules", "src/node_modules", false, true},
		{"node_modules", "a/b/c/node_modules", false, true},
		{".DS_Store", "deep/nested/.DS_Store", false, true},

		// must not match substrings
		{"node_modules", "not_node_modules", false, false},
		{"node_modules", "node_modules_extra", false, false},
		{".DS_Store", ".DS_Stores", false, false},
		{".DS_Store", "x.DS_Store", false, false},

		// case sensitive
		{"node_modules", "Node_Modules", false, false},
		{"node_modules", "NODE_MODULES", false, false},

		// empty last component should not match
		{"node_modules", "node_modules/", true, false},
	})
}

// TestNamePatternExactMatchAnchored tests exact patterns that are anchored
// (start with /) — these match the full relative path, not just the basename.
func TestNamePatternExactMatchAnchored(t *testing.T) {
	runNameTests(t, []nameMatchTest{
		// anchored exact match
		{"/Makefile", "Makefile", false, true},
		{"/Makefile", "src/Makefile", false, false},
		{"/build", "build", false, true},
		{"/build", "src/build", false, false},
	})
}

// TestNamePatternExactMatchDirectoryOnly tests exact patterns with trailing /
// that should only match directories.
func TestNamePatternExactMatchDirectoryOnly(t *testing.T) {
	runNameTests(t, []nameMatchTest{
		// directory-only pattern
		{"vendor/", "vendor", true, true},
		{"vendor/", "vendor", false, false}, // not a directory
		{"vendor/", "src/vendor", true, true},
		{"vendor/", "src/vendor", false, false},
	})
}

// TestNamePatternSuffixMatch exercises the matchSuffix fast-path:
// patterns like *.ext where only the leading * is a glob character.
func TestNamePatternSuffixMatch(t *testing.T) {
	runNameTests(t, []nameMatchTest{
		// basic suffix matches
		{"*.o", "foo.o", false, true},
		{"*.o", "bar.o", false, true},
		{"*.pyc", "module.pyc", false, true},
		{"*.min.js", "app.min.js", false, true},

		// nested paths
		{"*.o", "src/foo.o", false, true},
		{"*.o", "a/b/c/foo.o", false, true},
		{"*.pyc", "pkg/module.pyc", false, true},

		// must not match wrong extension
		{"*.o", "foo.obj", false, false},
		{"*.o", "foo.oo", false, false},
		{"*.pyc", "foo.py", false, false},

		// extension must be at the end
		{"*.o", "foo.o.bak", false, false},

		// file with just the suffix
		{"*.o", ".o", false, true},

		// multi-char suffix
		{"*.min.js", "lib.min.js", false, true},
		{"*.min.js", "lib.js", false, false},
		{"*.min.js", "min.js", false, false}, // * matches empty, ".min.js" != "min.js"
	})
}

// TestNamePatternSuffixMatchAnchored tests anchored suffix patterns.
func TestNamePatternSuffixMatchAnchored(t *testing.T) {
	runNameTests(t, []nameMatchTest{
		// an anchored name pattern is a single path component anchored to the
		// base directory, so it only matches single-segment paths and never
		// spans the '/' separator (matching git's behaviour for "/*.o")
		{"/*.o", "foo.o", false, true},
		{"/*.o", "src/foo.o", false, false}, // anchored: must not match nested
	})
}

// TestNamePatternComplexMatch exercises the matchComplex fallback:
// patterns that need full fnmatch (character classes, mid-pattern globs, etc.).
func TestNamePatternComplexMatch(t *testing.T) {
	runNameTests(t, []nameMatchTest{
		// character class
		{"*.[oa]", "foo.o", false, true},
		{"*.[oa]", "foo.a", false, true},
		{"*.[oa]", "foo.b", false, false},
		{"*.[oa]", "src/foo.o", false, true},

		// glob in the middle
		{"foo*.html", "foobar.html", false, true},
		{"foo*.html", "foo.html", false, true},
		{"foo*.html", "bar.html", false, false},

		// trailing glob (not a simple suffix since first char is not *)
		{"vmlinux*", "vmlinux", false, true},
		{"vmlinux*", "vmlinux.lds.S", false, true},
		{"vmlinux*", "not_vmlinux", false, false},

		// question mark
		{"test?.log", "test1.log", false, true},
		{"test?.log", "testA.log", false, true},
		{"test?.log", "test.log", false, false},
		{"test?.log", "test12.log", false, false},
	})
}

// TestNamePatternNegated tests that negation works correctly with all fast-path
// types (exact, suffix, complex).
func TestNamePatternNegated(t *testing.T) {
	// A negated pattern by itself won't produce an Ignore match.
	// We need a preceding ignore pattern, then the negation overrides it.
	_gitignore := "*.log\n!important.log\n"
	_ignore := gitignore.New(
		bytes.NewBufferString(_gitignore),
		"/base",
		nil,
	)

	tests := []struct {
		path   string
		isdir  bool
		ignore bool // expected Ignore() result
	}{
		// *.log matches → ignored (suffix fast-path)
		{"debug.log", false, true},
		{"src/error.log", false, true},
		// !important.log negates → included (exact fast-path)
		{"important.log", false, false},
		{"dir/important.log", false, false},
		// non-.log file → no match → not ignored
		{"readme.txt", false, false},
	}

	for _, tc := range tests {
		_match := _ignore.Relative(tc.path, tc.isdir)
		if tc.ignore {
			if _match == nil || !_match.Ignore() {
				t.Errorf(
					"path %q: expected ignored, got match=%v",
					tc.path, _match,
				)
			}
		} else {
			if _match != nil && _match.Ignore() {
				t.Errorf(
					"path %q: expected not ignored, got match=%v (Ignore=%v)",
					tc.path, _match, _match.Ignore(),
				)
			}
		}
	}
}

// TestNamePatternNegatedSuffix tests negation with suffix patterns.
func TestNamePatternNegatedSuffix(t *testing.T) {
	_gitignore := "*\n!*.go\n"
	_ignore := gitignore.New(
		bytes.NewBufferString(_gitignore),
		"/base",
		nil,
	)

	tests := []struct {
		path   string
		ignore bool
	}{
		// !*.go negates → included (suffix fast-path)
		{"main.go", false},
		{"src/util.go", false},
		// everything else matched by * → ignored
		{"readme.txt", true},
		{"Makefile", true},
	}

	for _, tc := range tests {
		_match := _ignore.Relative(tc.path, false)
		if tc.ignore {
			if _match == nil || !_match.Ignore() {
				t.Errorf("path %q: expected ignored", tc.path)
			}
		} else {
			if _match != nil && _match.Ignore() {
				t.Errorf("path %q: expected not ignored", tc.path)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// 2. path.Match() simplification tests (dead-code removal)
// ---------------------------------------------------------------------------

// TestPathPatternNonAnchored explicitly tests non-anchored path patterns
// to ensure the dead-code removal in path.Match() causes no regression.
// Non-anchored path patterns (e.g. "log/*.log" without leading /) should
// only match when the fnmatch pattern matches the full relative path.
func TestPathPatternNonAnchored(t *testing.T) {
	_gitignore := "log/*.log\nDocumentation/*.pdf\n"
	_ignore := gitignore.New(
		bytes.NewBufferString(_gitignore),
		"/base",
		nil,
	)

	tests := []struct {
		path  string
		isdir bool
		match bool
	}{
		// direct match
		{"log/test.log", false, true},
		{"log/error.log", false, true},
		{"Documentation/guide.pdf", false, true},

		// deeper nesting should NOT match (non-anchored path patterns
		// still require the full relative path to match the fnmatch)
		{"src/log/test.log", false, false},
		{"tools/perf/Documentation/perf.pdf", false, false},

		// wrong extension
		{"log/test.txt", false, false},
		{"Documentation/guide.doc", false, false},

		// directory mismatch
		{"logs/test.log", false, false},
	}

	for _, tc := range tests {
		_match := _ignore.Relative(tc.path, tc.isdir)
		got := _match != nil
		if got != tc.match {
			t.Errorf(
				"path %q: expected match=%v, got %v",
				tc.path, tc.match, got,
			)
		}
	}
}

// TestPathPatternAnchored tests anchored path patterns to ensure the
// path.Match() simplification doesn't affect them.
func TestPathPatternAnchored(t *testing.T) {
	_gitignore := "/src/vendor/\n/build/output/\n"
	_ignore := gitignore.New(
		bytes.NewBufferString(_gitignore),
		"/base",
		nil,
	)

	tests := []struct {
		path  string
		isdir bool
		match bool
	}{
		{"src/vendor", true, true},
		{"src/vendor", false, false}, // directory-only
		{"build/output", true, true},
		{"other/src/vendor", true, false}, // anchored, no deep match
	}

	for _, tc := range tests {
		_match := _ignore.Relative(tc.path, tc.isdir)
		got := _match != nil
		if got != tc.match {
			t.Errorf(
				"path %q isdir=%v: expected match=%v, got %v",
				tc.path, tc.isdir, tc.match, got,
			)
		}
	}
}

// ---------------------------------------------------------------------------
// 3. MatchIsDir sync.Map cache tests
// ---------------------------------------------------------------------------

// TestMatchIsDirBasic verifies that MatchIsDir returns the same results
// as Absolute for basic cases.
func TestMatchIsDirBasic(t *testing.T) {
	_dir, _ignore := directory(t)
	defer func(path string) { _ = os.RemoveAll(path) }(_dir)

	for _, _test := range _GITMATCHES {
		_path := filepath.Join(_dir, _test.Local())
		_match := _ignore.MatchIsDir(_path, _test.IsDir())

		if _test.Pattern == "" {
			if _match != nil {
				t.Errorf(
					"MatchIsDir(%q): expected nil match, got %v",
					_test.Path, _match,
				)
			}
		} else {
			if _match == nil {
				t.Errorf(
					"MatchIsDir(%q): expected match by %q, got nil",
					_test.Path, _test.Pattern,
				)
			} else if _match.String() != _test.Pattern {
				t.Errorf(
					"MatchIsDir(%q): expected pattern %q, got %q",
					_test.Path, _test.Pattern, _match.String(),
				)
			}
		}
	}
}

// TestMatchIsDirConcurrent exercises the sync.Map-based cache under
// concurrent access to verify there are no races.
func TestMatchIsDirConcurrent(t *testing.T) {
	_dir, _ignore := directory(t)
	defer func(path string) { _ = os.RemoveAll(path) }(_dir)

	// collect all test paths
	type testCase struct {
		absPath string
		isDir   bool
		pattern string
	}
	var cases []testCase
	for _, _test := range _GITMATCHES {
		cases = append(cases, testCase{
			absPath: filepath.Join(_dir, _test.Local()),
			isDir:   _test.IsDir(),
			pattern: _test.Pattern,
		})
	}

	// run MatchIsDir concurrently from many goroutines
	const goroutines = 16
	const iterations = 10
	var wg sync.WaitGroup
	errs := make(chan string, goroutines*len(cases)*iterations)

	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for iter := 0; iter < iterations; iter++ {
				for _, tc := range cases {
					_match := _ignore.MatchIsDir(tc.absPath, tc.isDir)
					if tc.pattern == "" {
						if _match != nil {
							errs <- fmt.Sprintf(
								"goroutine %d iter %d: path %q expected nil, got %v",
								id, iter, tc.absPath, _match,
							)
						}
					} else {
						if _match == nil {
							errs <- fmt.Sprintf(
								"goroutine %d iter %d: path %q expected %q, got nil",
								id, iter, tc.absPath, tc.pattern,
							)
						} else if _match.String() != tc.pattern {
							errs <- fmt.Sprintf(
								"goroutine %d iter %d: path %q expected %q, got %q",
								id, iter, tc.absPath, tc.pattern, _match.String(),
							)
						}
					}
				}
			}
		}(g)
	}

	wg.Wait()
	close(errs)

	for e := range errs {
		t.Error(e)
	}
}

// TestMatchIsDirCacheConsistency verifies that repeated calls to MatchIsDir
// with the same path return consistent results (cache hit path).
func TestMatchIsDirCacheConsistency(t *testing.T) {
	_dir, _ignore := directory(t)
	defer func(path string) { _ = os.RemoveAll(path) }(_dir)

	for _, _test := range _GITMATCHES {
		_path := filepath.Join(_dir, _test.Local())

		// call twice to exercise both cache-miss and cache-hit paths
		_first := _ignore.MatchIsDir(_path, _test.IsDir())
		_second := _ignore.MatchIsDir(_path, _test.IsDir())

		// both calls must return the same result
		if _first == nil && _second == nil {
			continue
		}
		if (_first == nil) != (_second == nil) {
			t.Errorf(
				"MatchIsDir(%q) inconsistency: first=%v, second=%v",
				_test.Path, _first, _second,
			)
			continue
		}
		if _first.String() != _second.String() {
			t.Errorf(
				"MatchIsDir(%q) inconsistency: first=%q, second=%q",
				_test.Path, _first.String(), _second.String(),
			)
		}
	}
}

// ---------------------------------------------------------------------------
// 4. Combined regression: full _GITMATCHES via Relative (exercises all
//    pattern types through the optimized code paths)
// ---------------------------------------------------------------------------

// TestAllMatchPatternsViaRelative runs the full _GITMATCHES suite through
// Relative() which directly invokes pattern.Match() on each pattern type.
// This serves as a broad regression test for all three optimizations.
func TestAllMatchPatternsViaRelative(t *testing.T) {
	_ignore := gitignore.New(
		bytes.NewBufferString(_GITMATCH),
		_GITBASE,
		nil,
	)

	for _, _test := range _GITMATCHES {
		_match := _ignore.Relative(_test.Local(), _test.IsDir())

		if _test.Pattern == "" {
			if _match != nil {
				t.Errorf(
					"Relative(%q, %v): expected nil, got pattern %q",
					_test.Path, _test.IsDir(), _match.String(),
				)
			}
		} else {
			if _match == nil {
				t.Errorf(
					"Relative(%q, %v): expected match by %q, got nil",
					_test.Path, _test.IsDir(), _test.Pattern,
				)
			} else if _match.String() != _test.Pattern {
				t.Errorf(
					"Relative(%q, %v): expected pattern %q, got %q",
					_test.Path, _test.IsDir(), _test.Pattern, _match.String(),
				)
			}
		}
	}
}
