package gitignore_test

import (
	"github.com/denormal/go-gitignore"
	"testing"
)

func TestPosition(t *testing.T) {
	// test the conversion of Positions to strings
	for _, _p := range _POSITIONS {
		_position := gitignore.Position{
			File:   _p.File,
			Line:   _p.Line,
			Column: _p.Column,
			Offset: _p.Offset,
		}

		// ensure the string representation of the Position is as expected
		_rtn := _position.String()
		if _rtn != _p.String {
			t.Errorf(
				"position mismatch; expected %q, got %q",
				_p.String, _rtn,
			)
		}
	}
} // TestPosition()
