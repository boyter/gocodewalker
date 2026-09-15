// SPDX-License-Identifier: MIT

package fnmatch

import (
	"strings"
	"testing"
	"time"
)

// TestUnterminatedClassDoesNotPanic covers the first defect fixed in this fork.
// rangematch consumed a class member and then read the byte after it without
// checking there was one, so a pattern of "[" plus a single multi byte rune ran
// off the end. The ASCII forms never panicked because one byte always left
// something behind, which is why this went unnoticed.
//
// An unterminated class matches nothing, which is what the ASCII forms already
// returned and what git does: `git check-ignore` with a .gitignore of "[a"
// ignores nothing at all, not even a file named "[a".
func TestUnterminatedClassDoesNotPanic(t *testing.T) {
	_patterns := []string{
		"[�",            // what the .gitignore lexer makes of "[" plus invalid UTF-8
		"[é",            // two byte rune
		"[中",            // three byte rune
		"[\U0001F600",   // four byte rune
		"[a-�",          // the same, as the upper end of a range
		"[�-",           //
		"[^中",           // negated
		"[!中",           //
		"[ab中",          // after some ASCII members
		"[",             // the ASCII forms, which never panicked
		"[a",            //
		"[ab",           //
		"[!",            //
		"**/[中",         // reached through a path pattern
		"a/[\U0001F600", //
		"[\\",           // trailing escape inside a class
		"[a-",           //
	}
	_subjects := []string{"", "0", "a", "中", "\xff", "a/b", "[a", "[中"}

	for _, _pattern := range _patterns {
		for _, _subject := range _subjects {
			for _, _flags := range []int{0, FNM_PATHNAME, FNM_NOESCAPE, FNM_PATHNAME | FNM_PERIOD} {
				_got, _panicked := matchRecovering(_pattern, _subject, _flags)
				if _panicked {
					t.Fatalf("Match(%q, %q, %d) panicked", _pattern, _subject, _flags)
				}
				if _got {
					t.Errorf("Match(%q, %q, %d) = true, want false: an unterminated class matches nothing",
						_pattern, _subject, _flags)
				}
			}
		}
	}
}

// TestPeriodEmptySubjectDoesNotPanic covers the second defect. Applying
// FNM_PERIOD after a "*" read the first byte of the subject without checking
// that the subject was non-empty. Nothing in gocodewalker passes FNM_PERIOD, so
// this was never reachable from the walker, but the package is general.
func TestPeriodEmptySubjectDoesNotPanic(t *testing.T) {
	for _, _case := range []struct {
		Pattern string
		Subject string
		Flags   int
	}{
		{"*", "", FNM_PERIOD},
		{"a*", "a", FNM_PERIOD},
		{"*", "", FNM_PERIOD | FNM_PATHNAME},
		{"a/*", "a/", FNM_PERIOD | FNM_PATHNAME},
		{"**", "", FNM_PERIOD},
	} {
		if _, _panicked := matchRecovering(_case.Pattern, _case.Subject, _case.Flags); _panicked {
			t.Errorf("Match(%q, %q, %d) panicked", _case.Pattern, _case.Subject, _case.Flags)
		}
	}
}

// TestStarRecursionIsBounded covers the third defect. The "*" recursion tried
// every remaining suffix of the subject at every "*", so the cost grew with the
// subject length raised to the number of stars. Before the memo this pattern
// took 1.1 seconds against a thousand characters and 9.1 seconds against two
// thousand; the shape of that growth is the point, so the test measures it
// rather than asserting one absolute duration.
func TestStarRecursionIsBounded(t *testing.T) {
	if testing.Short() {
		t.Skip("timing test")
	}

	const pattern = "*0*0*!*01"

	elapsed := func(n int) time.Duration {
		_subject := strings.Repeat("0", n) + "\x1a1"
		_start := time.Now()
		if Match(pattern, _subject, 0) {
			t.Fatalf("Match(%q, %d zeros) matched, expected no match", pattern, n)
		}

		return time.Since(_start)
	}

	// a generous ceiling: the unfixed version needed about nine seconds here
	if _took := elapsed(2000); _took > 2*time.Second {
		t.Fatalf("2000 characters took %v, the recursion is not bounded", _took)
	}

	// and the growth must be far below the cube it used to be. Doubling the
	// subject multiplied the old cost by about eight; allow a wide margin for a
	// loaded machine but not a whole order of magnitude.
	_small, _large := elapsed(1000), elapsed(2000)
	if _small > 0 && _large/_small > 6 {
		t.Errorf("doubling the subject multiplied the cost by %d (%v -> %v), which looks superlinear",
			_large/_small, _small, _large)
	}
}

// TestStarRecursionAnswersUnchanged is the other half of the memo: bounding the
// work must not change a single answer, including the ones that need the full
// search to come back true.
func TestStarRecursionAnswersUnchanged(t *testing.T) {
	for _, _case := range []struct {
		Pattern string
		Subject string
		Flags   int
		Want    bool
	}{
		{"*0*0*!*01", strings.Repeat("0", 200) + "\x1a1", 0, false},
		{"*a*b*c*d", "xxaxxbxxcxxd", 0, true},
		{"*a*b*c*d", "xxaxxbxxcxx", 0, false},
		{"*a*b*c*d*", "xxaxxbxxcxxdxx", 0, true},
		// a match that only succeeds on the last suffix tried
		{"*a*a*a*a", strings.Repeat("a", 64), 0, true},
		{"*a*a*a*b", strings.Repeat("a", 64), 0, false},
		// the same past the memo threshold, so the memoised path is the one
		// producing the answer
		{"*a*a*a*a", strings.Repeat("a", 512), 0, true},
		{"*a*a*a*b", strings.Repeat("a", 512), 0, false},
		{"*x*y*z", strings.Repeat("xy", 512) + "z", 0, true},
		// FNM_PATHNAME bounds each star to one component
		{"*a*b", strings.Repeat("a", 512) + "/b", FNM_PATHNAME, false},
		{"*/*b", strings.Repeat("a", 512) + "/xb", FNM_PATHNAME, true},
	} {
		if _got := Match(_case.Pattern, _case.Subject, _case.Flags); _got != _case.Want {
			t.Errorf("Match(%q, <%d chars>, %d) = %v, want %v",
				_case.Pattern, len(_case.Subject), _case.Flags, _got, _case.Want)
		}
	}
}

// TestMemoThresholdDoesNotChangeAnswers runs the same cases either side of the
// point where the memo is created, so that a change to memoThreshold cannot
// quietly alter behaviour.
func TestMemoThresholdDoesNotChangeAnswers(t *testing.T) {
	_original := memoThreshold
	defer func() { memoThreshold = _original }()

	_cases := []struct {
		Pattern string
		Subject string
		Flags   int
	}{
		{"*a*b*c", "zzazzbzzc", 0},
		{"*a*b*c", "zzazzbzz", 0},
		{"*.[oa]", "kernel.o", 0},
		{"*", "anything", 0},
		{"a*b*c/d", "axxbyyc/d", FNM_PATHNAME},
		{"*/*/*", "a/b/c", FNM_PATHNAME},
		{"*0*0*!*01", strings.Repeat("0", 64) + "\x1a1", 0},
	}

	for _, _case := range _cases {
		memoThreshold = 1 << 30 // never memoise
		_without := Match(_case.Pattern, _case.Subject, _case.Flags)
		memoThreshold = 0 // memoise from the first expansion
		_with := Match(_case.Pattern, _case.Subject, _case.Flags)

		if _without != _with {
			t.Errorf("Match(%q, %q, %d): %v without the memo, %v with it",
				_case.Pattern, _case.Subject, _case.Flags, _without, _with)
		}
	}
}

// matchRecovering reports what Match returned, and whether it panicked instead.
func matchRecovering(pattern, s string, flags int) (result bool, panicked bool) {
	defer func() {
		if recover() != nil {
			panicked = true
		}
	}()

	return Match(pattern, s, flags), false
}
