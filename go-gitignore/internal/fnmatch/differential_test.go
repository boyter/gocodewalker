// SPDX-License-Identifier: MIT

package fnmatch

import (
	"strings"
	"testing"
)

// outcome is what a matcher did: an answer, or a panic instead of one.
type outcome struct {
	result   bool
	panicked bool
}

func (o outcome) String() string {
	if o.panicked {
		return "panic"
	}

	return map[bool]string{true: "true", false: "false"}[o.result]
}

func run(fn func() bool) (o outcome) {
	defer func() {
		if recover() != nil {
			o = outcome{panicked: true}
		}
	}()

	return outcome{result: fn()}
}

// compare is the contract between this fork and the code it forked from: where
// upstream answered, the fork answers the same; where upstream panicked, the
// fork answers instead of panicking. The fork is never allowed to panic, and is
// never allowed to disagree with an answer upstream actually produced.
func compare(t *testing.T, pattern, s string, flags int) {
	t.Helper()

	_want := run(func() bool { return refMatch(pattern, s, flags) })
	_got := run(func() bool { return Match(pattern, s, flags) })

	if _got.panicked {
		t.Errorf("Match(%q, %q, %d) panicked; upstream gave %s", pattern, s, flags, _want)

		return
	}

	if !_want.panicked && _got.result != _want.result {
		t.Errorf("Match(%q, %q, %d) = %v, upstream gave %v",
			pattern, s, flags, _got.result, _want.result)
	}
}

var differentialFlags = []int{
	0,
	FNM_PATHNAME,
	FNM_NOESCAPE,
	FNM_PERIOD,
	FNM_CASEFOLD,
	FNM_LEADING_DIR,
	FNM_PATHNAME | FNM_PERIOD,
	FNM_PATHNAME | FNM_NOESCAPE,
	FNM_PATHNAME | FNM_PERIOD | FNM_CASEFOLD,
}

// TestMatchesUpstream is the table half of the differential: shapes chosen to
// reach each branch of the matcher rather than generated at random.
func TestMatchesUpstream(t *testing.T) {
	_patterns := []string{
		"", "*", "**", "?", "a", "a*", "*a", "*a*", "a?c", "a*b*c", "*a*b*c*d",
		".*", "*.o", "*.[oa]", "[abc]", "[a-z]", "[!a-z]", "[^a-z]", "[]]", "[-]",
		`a\*`, `\\`, `\`, `a\`, "[\\]", `[a\-z]`,
		"a/b", "a/*", "*/b", "a/*/b", "/a", "a/", "//", "a//b",
		"**/a", "a/**", "a/**/b", "*/*",
		// the unterminated classes, which panicked upstream
		"[", "[a", "[中", "[�", "[\U0001F600", "[a-中", "[!é",
		// non-ASCII generally
		"中*", "*中", "é?", "[中文]",
		// multi star, the shape the memo changes
		"*0*0*!*01", "*a*a*a*a", "*x*y*z*w*v",
	}
	_subjects := []string{
		"", "a", "ab", "abc", "a.o", "kernel.o", ".hidden", "..", ".",
		"a/b", "a/b/c", "/a", "a/", "//", "a//b", "[a", "[",
		"中文", "é", "\xff", "a\xffb", "\U0001F600",
		"000\x1a1", "xaxbxcxd", "aaaa", strings.Repeat("a", 40),
	}

	for _, _pattern := range _patterns {
		for _, _subject := range _subjects {
			for _, _flags := range differentialFlags {
				compare(t, _pattern, _subject, _flags)
			}
		}
	}
}

// FuzzMatchesUpstream is the generated half. It is the test that says the memo
// changed no answers, since anything it got wrong would show as a disagreement
// with the unmemoised original sitting next to it.
func FuzzMatchesUpstream(f *testing.F) {
	for _, _seed := range [][2]string{
		{"*", "a"},
		{"*a*b*c", "xaxbxc"},
		{"[a-z]", "m"},
		{"[中", "a"},
		{`a\*`, "a*"},
		{"a/*/b", "a/x/b"},
		{"*.[oa]", "x.o"},
		{"*0*0*!*01", "000\x1a1"},
		{".*", ".git"},
		{"[!]a]", "b"},
	} {
		f.Add(_seed[0], _seed[1], 0)
	}

	f.Fuzz(func(t *testing.T, pattern string, s string, flags int) {
		// the recursion is exponential upstream, and the reference is the
		// unfixed version, so keep the inputs small enough that comparing
		// against it stays affordable. The bound the fork puts on that
		// recursion is tested directly, not here.
		if len(pattern) > 24 || len(s) > 24 {
			t.Skip()
		}

		// only the defined flag bits; anything else is not a contract
		flags &= FNM_NOESCAPE | FNM_PATHNAME | FNM_PERIOD | FNM_LEADING_DIR | FNM_CASEFOLD

		compare(t, pattern, s, flags)
	})
}
