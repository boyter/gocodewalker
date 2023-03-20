package gitignore_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/denormal/go-gitignore"
)

// TestLexerNewLne tests the behavour of the gitignore.Lexer when the input
// data explicitly uses "\n" as the line separator
func TestLexerNewLine(t *testing.T) {
	// split the test content into lines
	//		- ensure we handle "\n" and "\r" correctly
	//		- since this test file is written on a system that uses "\n"
	//		  to designate end of line, this should be unnecessary, but it's
	//		  possible for the file line endings to be converted outside of
	//		  this repository, so we are thorough here to ensure the test
	//		  works as expected everywhere
	_content := strings.Split(_GITIGNORE, "\n")
	for _i := 0; _i < len(_content); _i++ {
		_content[_i] = strings.TrimSuffix(_content[_i], "\r")
	}

	// perform the Lexer test with input explicitly separated by "\n"
	lexer(t, _content, "\n", _GITTOKENS, nil)
} // TestLexerNewLine()

// TestLexerCarriageReturn tests the behavour of the gitignore.Lexer when the
// input data explicitly uses "\r\n" as the line separator
func TestLexerCarriageReturn(t *testing.T) {
	// split the test content into lines
	//		- see above
	_content := strings.Split(_GITIGNORE, "\n")
	for _i := 0; _i < len(_content); _i++ {
		_content[_i] = strings.TrimSuffix(_content[_i], "\r")
	}

	// perform the Lexer test with input explicitly separated by "\r\n"
	lexer(t, _content, "\r\n", _GITTOKENS, nil)
} // TestLexerCarriageReturn()

func TestLexerInvalidNewLine(t *testing.T) {
	// perform the Lexer test with invalid input separated by "\n"
	//		- the source content is manually constructed with "\n" as EOL
	_content := strings.Split(_GITINVALID, "\n")
	lexer(t, _content, "\n", _TOKENSINVALID, gitignore.CarriageReturnError)
} // TestLexerInvalidNewLine()

func TestLexerInvalidCarriageReturn(t *testing.T) {
	// perform the Lexer test with invalid input separated by "\n"
	//		- the source content is manually constructed with "\n" as EOL
	_content := strings.Split(_GITINVALID, "\n")
	lexer(t, _content, "\r\n", _TOKENSINVALID, gitignore.CarriageReturnError)
} // TestLexerInvalidCarriageReturn()

func lexer(t *testing.T, lines []string, eol string, tokens []token, e error) {
	// create a temporary .gitignore
	_buffer, _err := buffer(strings.Join(lines, eol))
	if _err != nil {
		t.Fatalf("unable to create temporary .gitignore: %s", _err.Error())
	}

	// ensure we have a non-nil Lexer instance
	_lexer := gitignore.NewLexer(_buffer)
	if _lexer == nil {
		t.Error("expected non-nil Lexer instance; nil found")
	}

	// ensure the stream of tokens is as we expect
	for _, _expected := range tokens {
		_position := _lexer.Position()

		// ensure the string form of the Lexer reports the correct position
		_string := fmt.Sprintf("%d:%d", _position.Line, _position.Column)
		if _lexer.String() != _string {
			t.Errorf(
				"lexer string mismatch; expected %q, got %q",
				_string, _position.String(),
			)
		}

		// extract the next token from the lexer
		_got, _err := _lexer.Next()

		// ensure we did not receive an error and the token is as expected
		if _err != nil {
			// if we expect an error during processing, check to see if
			// the received error is as expected
			//			if !_err.Is(e) {
			if _err.Underlying() != e {
				t.Fatalf(
					"unable to retrieve expected token; %s at %s",
					_err.Error(), pos(_err.Position()),
				)
			}
		}

		// did we receive a token?
		if _got == nil {
			t.Fatalf("expected token at %s; none found", _lexer)
		} else if _got.Type != _expected.Type {
			t.Fatalf(
				"token type mismatch; expected type %d, got %d [%s]",
				_expected.Type, _got.Type, _got,
			)
		} else if _got.Name() != _expected.Name {
			t.Fatalf(
				"token name mismatch; expected name %q, got %q [%s]",
				_expected.Name, _got.Name(), _got,
			)
		} else {
			// ensure the extracted token string matches expectation
			//		- we handle EOL separately, since it can change based
			//		  on the end of line sequence of the input file
			_same := _got.Token() == _expected.Token
			if _got.Type == gitignore.EOL {
				_same = _got.Token() == eol
			}
			if !_same {
				t.Fatalf(
					"token value mismatch; expected name %q, got %q [%s]",
					_expected.Token, _got.Token(), _got,
				)
			}

			// ensure the token position matches the original lexer position
			if !coincident(_got.Position, _position) {
				t.Fatalf(
					"token position mismatch for %s; expected %s, got %s",
					_got, pos(_position), pos(_got.Position),
				)
			}

			// ensure the token position matches the expected position
			//		- since we will be testing with different line endings, we
			//		  have to choose the correct offset
			_position := gitignore.Position{
				File:   "",
				Line:   _expected.Line,
				Column: _expected.Column,
				Offset: _expected.NewLine,
			}
			if eol == "\r\n" {
				_position.Offset = _expected.CarriageReturn
			}
			if !coincident(_got.Position, _position) {
				t.Log(pos(_got.Position) + "\t" + _got.String())
				t.Fatalf(
					"token position mismatch; expected %s, got %s",
					pos(_position), pos(_got.Position),
				)
			}
		}
	}

	// ensure there are no more tokens
	_next, _err := _lexer.Next()
	if _err != nil {
		t.Errorf("unexpected error on end of token test: %s", _err.Error())
	} else if _next == nil {
		t.Errorf("unexpected nil token at end of test")
	} else if _next.Type != gitignore.EOF {
		t.Errorf(
			"token type mismatch; expected type %d, got %d [%s]",
			gitignore.EOF, _next.Type, _next,
		)
	}
} // TestLexer()
