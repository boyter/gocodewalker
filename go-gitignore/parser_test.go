package gitignore_test

import (
	"strings"
	"testing"

	"github.com/denormal/go-gitignore"
)

type parsetest struct {
	good     int
	bad      int
	position []gitignore.Position
	failures []gitignore.Error
	errors   func(gitignore.Error) bool
} // parsetest{}

// TestParser tests the behaviour of gitignore.Parser
func TestParser(t *testing.T) {
	_test := &parsetest{good: _GITPATTERNS, bad: _GITBADPATTERNS}
	_test.position = make([]gitignore.Position, 0)

	// record the position of encountered errors
	_test.errors = func(e gitignore.Error) bool {
		_test.position = append(_test.position, e.Position())
		return true
	}

	// run this parser test
	parse(t, _test)
} // TestParser()

// TestParserError tests the behaviour of the gitignore.Parser with an error
// handler that returns false on receiving an error
func TestParserError(t *testing.T) {
	_test := &parsetest{good: _GITPATTERNSFALSE, bad: _GITBADPATTERNSFALSE}
	_test.position = make([]gitignore.Position, 0)

	// record the position of encountered errors
	//		- return false to stop parsing
	_test.errors = func(e gitignore.Error) bool {
		_test.position = append(_test.position, e.Position())
		return false
	}

	// run this parser test
	parse(t, _test)
} // TestParserError()

func TestParserInvalid(t *testing.T) {
	_test := &parsetest{good: _GITINVALIDPATTERNS, bad: _GITINVALIDERRORS}
	_test.position = make([]gitignore.Position, 0)

	// record the position of encountered errors
	_test.errors = func(e gitignore.Error) bool {
		_test.position = append(_test.position, e.Position())
		_test.failures = append(_test.failures, e)
		return true
	}

	// run this parser test
	invalidparse(t, _test)
} // TestParserInvalid()

func TestParserInvalidFalse(t *testing.T) {
	_test := &parsetest{
		good: _GITINVALIDPATTERNSFALSE,
		bad:  _GITINVALIDERRORSFALSE,
	}
	_test.position = make([]gitignore.Position, 0)

	// record the position of encountered errors
	_test.errors = func(e gitignore.Error) bool {
		_test.position = append(_test.position, e.Position())
		_test.failures = append(_test.failures, e)
		return false
	}

	// run this parser test
	invalidparse(t, _test)
} // TestParserInvalidFalse()

func parse(t *testing.T, test *parsetest) {
	// create a temporary .gitignore
	_buffer, _err := buffer(_GITIGNORE)
	if _err != nil {
		t.Fatalf("unable to create temporary .gitignore: %s", _err.Error())
	}

	// ensure we have a non-nil Parser instance
	_parser := gitignore.NewParser(_buffer, test.errors)
	if _parser == nil {
		t.Error("expected non-nil Parser instance; nil found")
	}

	// before we parse, what position do we have?
	_position := _parser.Position()
	if !coincident(_position, _BEGINNING) {
		t.Errorf(
			"beginning position mismatch; expected %s, got %s",
			pos(_BEGINNING), pos(_position),
		)
	}

	// attempt to parse the .gitignore
	_patterns := _parser.Parse()

	// ensure we encountered the expected bad patterns
	if len(test.position) != test.bad {
		t.Errorf(
			"parse error mismatch; expected %d errors, got %d",
			test.bad, len(test.position),
		)
	} else {
		// ensure the bad pattern positions are correct
		for _i := 0; _i < test.bad; _i++ {
			_got := test.position[_i]
			_expected := _GITBADPOSITION[_i]

			if !coincident(_got, _expected) {
				t.Errorf(
					"bad pattern position mismatch; expected %q, got %q",
					pos(_expected), pos(_got),
				)
			}
		}
	}

	// ensure we encountered the right number of good patterns
	if len(_patterns) != test.good {
		t.Errorf(
			"parse pattern mismatch; expected %d patterns, got %d",
			test.good, len(_patterns),
		)
	} else {
		// ensure the good pattern positions are correct
		for _i := 0; _i < len(_patterns); _i++ {
			_got := _patterns[_i].Position()
			_expected := _GITPOSITION[_i]

			if !coincident(_got, _expected) {
				t.Errorf(
					"pattern position mismatch; expected %q, got %q",
					pos(_expected), pos(_got),
				)
			}
		}

		// ensure the retrieved patterns are correct
		//		- we check the string form of the pattern against the respective
		//	      lines from the .gitignore
		//		- we must special-case patterns that end in whitespace that
		//		  can be ignored (i.e. it's not escaped)
		_lines := strings.Split(_GITIGNORE, "\n")
		for _i := 0; _i < len(_patterns); _i++ {
			_pattern := _patterns[_i]
			_got := _pattern.String()
			_line := _pattern.Position().Line
			_expected := _lines[_line-1]

			if _got != _expected {
				// if the two strings aren't the same, then check to see if
				// the difference is trailing whitespace
				//		- the expected string may have whitespace, while the
				//		  pattern string does not
				//		- patterns have their trailing whitespace removed, so
				//	    - we perform this check here, since it's possible for
				//		  a pattern to end in a whitespace character (e.g. '\ ')
				//		  and we don't want to be too heavy handed with our
				//		  removal of whitespace
				//		- only do this check for non-comments
				if !strings.HasPrefix(_expected, "#") {
					_new := strings.TrimRight(_expected, " \t")
					if _new == _got {
						continue
					}
				}
				t.Errorf(
					"pattern mismatch; expected %q, got %q at %s",
					_expected, _got, pos(_pattern.Position()),
				)
			}
		}
	}
} // parse()

func invalidparse(t *testing.T, test *parsetest) {
	_buffer, _err := buffer(_GITINVALID)
	if _err != nil {
		t.Fatalf("unable to create temporary .gitignore: %s", _err.Error())
	}

	// create the parser instance
	_parser := gitignore.NewParser(_buffer, test.errors)
	if _parser == nil {
		t.Error("expected non-nil Parser instance; nil found")
	}

	// attempt to parse the .gitignore
	_patterns := _parser.Parse()

	// ensure we have the correct number of errors encountered
	if len(test.failures) != test.bad {
		t.Fatalf(
			"unexpected invalid parse errors; expected %d, got %d",
			test.bad, len(test.failures),
		)
	} else {
		for _i := 0; _i < test.bad; _i++ {
			_expected := _GITINVALIDERROR[_i]
			_got := test.failures[_i]

			// is this error the same as expected?
			//			if !_got.Is(_expected) {
			if _got.Underlying() != _expected {
				t.Fatalf(
					"unexpected invalid parse error; expected %q, got %q",
					_expected.Error(), _got.Error(),
				)
			}
		}
	}

	// ensure we have the correct number of patterns
	if len(_patterns) != test.good {
		t.Fatalf(
			"unexpected invalid parse patterns; expected %d, got %d",
			test.good, len(_patterns),
		)
	} else {
		for _i := 0; _i < test.good; _i++ {
			_expected := _GITINVALIDPATTERN[_i]
			_got := _patterns[_i]

			// is this pattern the same as expected?
			if _got.String() != _expected {
				t.Fatalf(
					"unexpected invalid parse pattern; expected %q, got %q",
					_expected, _got,
				)
			}
		}
	}
} // invalidparse()
