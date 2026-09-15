// SPDX-License-Identifier: MIT

package fnmatch

// This file carries github.com/danwakefield/fnmatch as it was before this fork,
// verbatim apart from the identifiers being renamed, so that the differential
// test has the original behaviour to compare against without depending on the
// module. It was generated from the vendored upstream source and was checked,
// while that dependency was still present, to agree with the real thing on
// every input a 200 million execution fuzz could find, panics included.
//
// Do not fix anything here. Its whole purpose is to be wrong in the three ways
// the package documentation describes, so that the tests can prove the fork is
// right where upstream was wrong and identical everywhere else.

import (
	"unicode"
	"unicode/utf8"
)

func refUnpackRune(str *string) rune {
	rune, size := utf8.DecodeRuneInString(*str)
	*str = (*str)[size:]
	return rune
}

// Matches the pattern against the string, with the given flags,
// and returns true if the match is successful.
// This function should match fnmatch.3 as closely as possible.
func refMatch(pattern, s string, flags int) bool {
	// The implementation for this function was patterned after the BSD fnmatch.c
	// source found at http://src.gnu-darwin.org/src/contrib/csup/fnmatch.c.html
	noescape := (flags&FNM_NOESCAPE != 0)
	pathname := (flags&FNM_PATHNAME != 0)
	period := (flags&FNM_PERIOD != 0)
	leadingdir := (flags&FNM_LEADING_DIR != 0)
	casefold := (flags&FNM_CASEFOLD != 0)
	// the following is some bookkeeping that the original fnmatch.c implementation did not do
	// We are forced to do this because we're not keeping indexes into C strings but rather
	// processing utf8-encoded strings. Use a custom unpacker to maintain our state for us
	sAtStart := true
	sLastAtStart := true
	sLastSlash := false
	sLastUnpacked := rune(0)
	unpackS := func() rune {
		sLastSlash = (sLastUnpacked == '/')
		sLastUnpacked = refUnpackRune(&s)
		sLastAtStart = sAtStart
		sAtStart = false
		return sLastUnpacked
	}
	for len(pattern) > 0 {
		c := refUnpackRune(&pattern)
		switch c {
		case '?':
			if len(s) == 0 {
				return false
			}
			sc := unpackS()
			if pathname && sc == '/' {
				return false
			}
			if period && sc == '.' && (sLastAtStart || (pathname && sLastSlash)) {
				return false
			}
		case '*':
			// collapse multiple *'s
			// don't use unpackRune here, the only char we care to detect is ASCII
			for len(pattern) > 0 && pattern[0] == '*' {
				pattern = pattern[1:]
			}
			if period && s[0] == '.' && (sAtStart || (pathname && sLastUnpacked == '/')) {
				return false
			}
			// optimize for patterns with * at end or before /
			if len(pattern) == 0 {
				if pathname {
					return leadingdir || (refStrchr(s, '/') == -1)
				} else {
					return true
				}
				// upstream has one more return here, unreachable after the if
				// and the else above both return. Dropped only because go vet
				// objects; dead code cannot affect what this reference answers.
			} else if pathname && pattern[0] == '/' {
				offset := refStrchr(s, '/')
				if offset == -1 {
					return false
				} else {
					// we already know our pattern and string have a /, skip past it
					s = s[offset:] // use unpackS here to maintain our bookkeeping state
					unpackS()
					pattern = pattern[1:] // we know / is one byte long
					break
				}
			}
			// general case, recurse
			for test := s; len(test) > 0; refUnpackRune(&test) {
				// I believe the (flags &^ FNM_PERIOD) is a bug when FNM_PATHNAME is specified
				// but this follows exactly from how fnmatch.c implements it
				if refMatch(pattern, test, (flags &^ FNM_PERIOD)) {
					return true
				} else if pathname && test[0] == '/' {
					break
				}
			}
			return false
		case '[':
			if len(s) == 0 {
				return false
			}
			if pathname && s[0] == '/' {
				return false
			}
			sc := unpackS()
			if !refRangematch(&pattern, sc, flags) {
				return false
			}
		case '\\':
			if !noescape {
				if len(pattern) > 0 {
					c = refUnpackRune(&pattern)
				}
			}
			fallthrough
		default:
			if len(s) == 0 {
				return false
			}
			sc := unpackS()
			switch {
			case sc == c:
			case casefold && unicode.ToLower(sc) == unicode.ToLower(c):
			default:
				return false
			}
		}
	}
	return len(s) == 0 || (leadingdir && s[0] == '/')
}

func refRangematch(pattern *string, test rune, flags int) bool {
	if len(*pattern) == 0 {
		return false
	}
	casefold := (flags&FNM_CASEFOLD != 0)
	noescape := (flags&FNM_NOESCAPE != 0)
	if casefold {
		test = unicode.ToLower(test)
	}
	var negate, matched bool
	if (*pattern)[0] == '^' || (*pattern)[0] == '!' {
		negate = true
		(*pattern) = (*pattern)[1:]
	}
	for !matched && len(*pattern) > 1 && (*pattern)[0] != ']' {
		c := refUnpackRune(pattern)
		if !noescape && c == '\\' {
			if len(*pattern) > 1 {
				c = refUnpackRune(pattern)
			} else {
				return false
			}
		}
		if casefold {
			c = unicode.ToLower(c)
		}
		if (*pattern)[0] == '-' && len(*pattern) > 1 && (*pattern)[1] != ']' {
			refUnpackRune(pattern) // skip the -
			c2 := refUnpackRune(pattern)
			if !noescape && c2 == '\\' {
				if len(*pattern) > 0 {
					c2 = refUnpackRune(pattern)
				} else {
					return false
				}
			}
			if casefold {
				c2 = unicode.ToLower(c2)
			}
			// this really should be more intelligent, but it looks like
			// fnmatch.c does simple int comparisons, therefore we will as well
			if c <= test && test <= c2 {
				matched = true
			}
		} else if c == test {
			matched = true
		}
	}
	// skip past the rest of the pattern
	ok := false
	for !ok && len(*pattern) > 0 {
		c := refUnpackRune(pattern)
		if c == '\\' && len(*pattern) > 0 {
			refUnpackRune(pattern)
		} else if c == ']' {
			ok = true
		}
	}
	return ok && matched != negate
}

// define strchr because strings.Index() seems a bit overkill
// returns the index of c in s, or -1 if there is no match
func refStrchr(s string, c rune) int {
	for i, sc := range s {
		if sc == c {
			return i
		}
	}
	return -1
}
