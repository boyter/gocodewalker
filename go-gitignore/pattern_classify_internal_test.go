// SPDX-License-Identifier: MIT

package gitignore

import (
	"strings"
	"testing"

	"github.com/boyter/gocodewalker/go-gitignore/internal/fnmatch"
)

// classified builds the name matcher classifyGlob would produce for fn, without
// going near the lexer. The lexer rewrites some inputs on their way to becoming
// a pattern (a NUL ends the line, trailing whitespace is dropped), so feeding it
// a fuzzed string and then comparing against fnmatch of that same string
// compares two different patterns. Starting from the fnmatch expression instead
// tests the invariant that actually matters: for a given expression, the fast
// path has to agree with fnmatch.
func classified(fn string) *name {
	n := &name{}
	n._fnmatch = fn
	n._matchType, n._literal = classifyGlob(fn)

	return n
}

// FuzzClassifyGlobAgreesWithFnmatch is the differential test for the classifier
// behind every name pattern. matchExact, matchSuffix, matchPrefix, matchContains
// and matchAny each replace an fnmatch call with a byte comparison, and each is
// only valid while it gives the same answer fnmatch would.
//
// This is how the invalid-UTF-8 divergence should have been caught: the existing
// table-driven differential test only ever fed the classifier valid UTF-8.
func FuzzClassifyGlobAgreesWithFnmatch(f *testing.F) {
	for _, _seed := range [][2]string{
		// the divergence this exists to pin: U+FFFD in the pattern matches any
		// invalid byte under fnmatch, which no byte comparison reproduces
		{"\xd7", "\xff"},
		{"*\xd7*", "a\xffb"},
		{"�", "\xfe"},
		// ordinary traffic through each bucket
		{"node_modules", "node_modules"},
		{"*.o", "kernel.o"},
		{".*", ".gitignore"},
		{"*test*", "my_test_file"},
		{"*", "anything"},
		// the ones that look like a fast path but are not
		{`abc\*`, "abc*"},
		{"**", "a"},
		{"*.[oa]", "x.o"},
		{"a?c", "abc"},
	} {
		f.Add(_seed[0], _seed[1])
	}

	f.Fuzz(func(t *testing.T, fn string, target string) {
		// a name pattern is only ever matched against one path component
		if strings.ContainsAny(target, "/") {
			t.Skip()
		}

		// fnmatch backtracks over every '*' in the expression, and its cost
		// grows with the cube of the target length: a nine character expression
		// against a two thousand character name takes about nine seconds. That
		// is a real pre-existing problem in the vendored fnmatch, but it is not
		// this classifier's, and left unbounded it is all the fuzzer finds.
		if len(fn) > 64 || len(target) > 128 {
			t.Skip()
		}

		// an unterminated character class panics inside the vendored fnmatch.
		// That is a real pre-existing bug, but it is a bug in fnmatch's parser
		// rather than a disagreement with the fast paths, and it would mask
		// everything else this fuzzer is looking for.
		if strings.ContainsAny(fn, "[\\") {
			t.Skip()
		}

		_got := classified(fn).Match(target, false)
		_want := fnmatch.Match(fn, target, 0)

		if _got != _want {
			_type, _literal := classifyGlob(fn)
			t.Errorf(
				"expression %q against %q: fast path %v, fnmatch %v (classified as %v, literal %q)",
				fn, target, _got, _want, _type, _literal,
			)
		}
	})
}

// TestClassifyGlobBuckets pins which bucket each shape of expression lands in,
// so that a change to the classifier that happens to stay correct is still
// visible as a change in what takes a fast path.
func TestClassifyGlobBuckets(t *testing.T) {
	for _, _case := range []struct {
		Expression string
		Type       matchType
		Literal    string
	}{
		{"node_modules", matchExact, "node_modules"},
		{"", matchExact, ""},
		{"*", matchAny, ""},
		{"*.o", matchSuffix, ".o"},
		{".*", matchPrefix, "."},
		{"*test*", matchContains, "test"},
		{"**", matchComplex, ""},
		{"a?c", matchComplex, ""},
		{"*.[oa]", matchComplex, ""},
		{`abc\*`, matchComplex, ""},
		// invalid UTF-8 has to reach fnmatch whatever shape it is in
		{"\xd7", matchComplex, ""},
		{"*\xd7", matchComplex, ""},
		{"\xd7*", matchComplex, ""},
		{"*\xd7*", matchComplex, ""},
	} {
		_type, _literal := classifyGlob(_case.Expression)
		if _type != _case.Type || _literal != _case.Literal {
			t.Errorf(
				"expression %q: classified as %v literal %q, want %v literal %q",
				_case.Expression, _type, _literal, _case.Type, _case.Literal,
			)
		}
	}
}
