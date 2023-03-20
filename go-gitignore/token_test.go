package gitignore_test

import (
	"fmt"
	"testing"

	"github.com/denormal/go-gitignore"
)

func TestToken(t *testing.T) {
	for _, _test := range _TOKENS {
		// create the token
		_position := gitignore.Position{
			File:   "file",
			Line:   _test.Line,
			Column: _test.Column,
			Offset: _test.NewLine,
		}
		_token := gitignore.NewToken(
			_test.Type, []rune(_test.Token), _position,
		)

		// ensure we have a non-nil token
		if _token == nil {
			t.Errorf(
				"unexpected nil Token for type %d %q", _test.Type, _test.Name,
			)
			continue
		}

		// ensure the token type match
		if _token.Type != _test.Type {
			// if we have a bad token, then we accept token types that
			// are outside the range of permitted token values
			if _token.Type == gitignore.BAD {
				if _test.Type < gitignore.ILLEGAL ||
					_test.Type > gitignore.BAD {
					goto NAME
				}
			}

			// otherwise, we have a type mismatch
			t.Errorf(
				"token type mismatch for %q; expected %d, got %d",
				_test.Name, _test.Type, _token.Type,
			)
			continue
		}

	NAME:
		// ensure the token name match
		if _token.Name() != _test.Name {
			t.Errorf(
				"token name mismatch for type %d; expected %s, got %s",
				_test.Type, _test.Name, _token.Name(),
			)
			continue
		}

		// ensure the positions are the same
		if !coincident(_position, _token.Position) {
			t.Errorf(
				"token position mismatch; expected %s, got %s",
				pos(_position), pos(_token.Position),
			)
			continue
		}

		// ensure the string form of the token is as expected
		_string := fmt.Sprintf(
			"%s: %s %q", _position, _test.Name, _test.Token,
		)
		if _string != _token.String() {
			t.Errorf(
				"token string mismatch; expected %q, got %q",
				_string, _token.String(),
			)
		}
	}
} // TestToken()
