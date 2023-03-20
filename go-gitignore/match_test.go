package gitignore_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/denormal/go-gitignore"
)

func TestMatch(t *testing.T) {
	// we need to populate a directory with the match test files
	//		- this is to permit GitIgnore.Match() to correctly resolve
	//		  absolute path names
	_dir, _ignore := directory(t)
	defer os.RemoveAll(_dir)

	// perform the path matching
	//		- first we test absolute paths
	_cb := func(path string, isdir bool) gitignore.Match {
		_path := filepath.Join(_dir, path)
		return _ignore.Match(_path)
	}
	for _, _test := range _GITMATCHES {
		do(t, _cb, _test)
	}

	// now, attempt relative path matching
	//		- to do this, we need to change the working directory
	_cwd, _err := os.Getwd()
	if _err != nil {
		t.Fatalf("unable to retrieve working directory: %s", _err.Error())
	}
	_err = os.Chdir(_dir)
	if _err != nil {
		t.Fatalf("unable to chdir into temporary directory: %s", _err.Error())
	}
	defer os.Chdir(_cwd)

	// perform the relative path tests
	_cb = func(path string, isdir bool) gitignore.Match {
		return _ignore.Match(path)
	}
	for _, _test := range _GITMATCHES {
		do(t, _cb, _test)
	}

	// perform absolute path tests with paths not under the same root
	// directory as the GitIgnore we are testing
	_new, _ := directory(t)
	defer os.RemoveAll(_new)

	for _, _test := range _GITMATCHES {
		_path := filepath.Join(_new, _test.Local())
		_match := _ignore.Match(_path)
		if _match != nil {
			t.Fatalf("unexpected match; expected nil, got %v", _match)
		}
	}

	// ensure Match() behaves as expected if the absolute path cannot
	// be determined
	//		- we do this by choosing as our working directory a path
	//		  that this process does not have permission to
	_dir, _err = dir(nil)
	if _err != nil {
		t.Fatalf("unable to create temporary directory: %s", _err.Error())
	}
	defer os.RemoveAll(_dir)

	_err = os.Chdir(_dir)
	if _err != nil {
		t.Fatalf("unable to chdir into temporary directory: %s", _err.Error())
	}
	defer os.Chdir(_cwd)

	// remove permission from the temporary directory
	_err = os.Chmod(_dir, 0)
	if _err != nil {
		t.Fatalf(
			"unable to modify temporary directory permissions: %s: %s",
			_dir, _err.Error(),
		)
	}

	// now perform the match tests and ensure an error is returned
	for _, _test := range _GITMATCHES {
		_match := _ignore.Match(_test.Local())
		if _match != nil {
			t.Fatalf("unexpected match; expected nil, got %v", _match)
		}
	}
} // TestMatch()

func TestIgnore(t *testing.T) {
	// we need to populate a directory with the match test files
	//		- this is to permit GitIgnore.Ignore() to correctly resolve
	//		  absolute path names
	_dir, _ignore := directory(t)
	defer os.RemoveAll(_dir)

	// perform the path matching
	//		- first we test absolute paths
	for _, _test := range _GITMATCHES {
		_path := filepath.Join(_dir, _test.Local())
		_rtn := _ignore.Ignore(_path)
		if _rtn != _test.Ignore {
			t.Errorf(
				"ignore mismatch for %q; expected %v, got %v",
				_path, _test.Ignore, _rtn,
			)
		}
	}

	// now, attempt relative path matching
	//		- to do this, we need to change the working directory
	_cwd, _err := os.Getwd()
	if _err != nil {
		t.Fatalf("unable to retrieve working directory: %s", _err.Error())
	}
	_err = os.Chdir(_dir)
	if _err != nil {
		t.Fatalf("unable to chdir into temporary directory: %s", _err.Error())
	}
	defer os.Chdir(_cwd)

	// perform the relative path tests
	for _, _test := range _GITMATCHES {
		_rtn := _ignore.Ignore(_test.Local())
		if _rtn != _test.Ignore {
			t.Errorf(
				"ignore mismatch for %q; expected %v, got %v",
				_test.Path, _test.Ignore, _rtn,
			)
		}
	}

	// perform absolute path tests with paths not under the same root
	// directory as the GitIgnore we are testing
	_new, _ := directory(t)
	defer os.RemoveAll(_new)

	for _, _test := range _GITMATCHES {
		_path := filepath.Join(_new, _test.Local())
		_ignore := _ignore.Ignore(_path)
		if _ignore {
			t.Fatalf("unexpected ignore for %q", _path)
		}
	}
} // TestIgnore()

func TestInclude(t *testing.T) {
	// we need to populate a directory with the match test files
	//		- this is to permit GitIgnore.Include() to correctly resolve
	//		  absolute path names
	_dir, _ignore := directory(t)
	defer os.RemoveAll(_dir)

	// perform the path matching
	//		- first we test absolute paths
	for _, _test := range _GITMATCHES {
		_path := filepath.Join(_dir, _test.Local())
		_rtn := _ignore.Include(_path)
		if _rtn == _test.Ignore {
			t.Errorf(
				"include mismatch for %q; expected %v, got %v",
				_path, !_test.Ignore, _rtn,
			)
		}
	}

	// now, attempt relative path matching
	//		- to do this, we need to change the working directory
	_cwd, _err := os.Getwd()
	if _err != nil {
		t.Fatalf("unable to retrieve working directory: %s", _err.Error())
	}
	_err = os.Chdir(_dir)
	if _err != nil {
		t.Fatalf("unable to chdir into temporary directory: %s", _err.Error())
	}
	defer os.Chdir(_cwd)

	// perform the relative path tests
	for _, _test := range _GITMATCHES {
		_rtn := _ignore.Include(_test.Local())
		if _rtn == _test.Ignore {
			t.Errorf(
				"include mismatch for %q; expected %v, got %v",
				_test.Path, !_test.Ignore, _rtn,
			)
		}
	}

	// perform absolute path tests with paths not under the same root
	// directory as the GitIgnore we are testing
	_new, _ := directory(t)
	defer os.RemoveAll(_new)

	for _, _test := range _GITMATCHES {
		_path := filepath.Join(_new, _test.Local())
		_include := _ignore.Include(_path)
		if !_include {
			t.Fatalf("unexpected include for %q", _path)
		}
	}
} // TestInclude()

func TestMatchAbsolute(t *testing.T) {
	// create a temporary .gitignore
	_buffer, _err := buffer(_GITMATCH)
	if _err != nil {
		t.Fatalf("unable to create temporary .gitignore: %s", _err.Error())
	}

	// ensure we can run New()
	//		- ensure we encounter no errors
	_position := []gitignore.Position{}
	_error := func(e gitignore.Error) bool {
		_position = append(_position, e.Position())
		return true
	}

	// ensure we have a non-nil GitIgnore instance
	_ignore := gitignore.New(_buffer, _GITBASE, _error)
	if _ignore == nil {
		t.Error("expected non-nil GitIgnore instance; nil found")
	}

	// ensure we encountered the right number of errors
	if len(_position) != _GITBADMATCHPATTERNS {
		t.Errorf(
			"match error mismatch; expected %d errors, got %d",
			_GITBADMATCHPATTERNS, len(_position),
		)
	}

	// perform the absolute path matching
	_cb := func(path string, isdir bool) gitignore.Match {
		_path := filepath.Join(_GITBASE, path)
		return _ignore.Absolute(_path, isdir)
	}
	for _, _test := range _GITMATCHES {
		do(t, _cb, _test)
	}

	// perform absolute path tests with paths not under the same root
	// directory as the GitIgnore we are testing
	_new, _ := directory(t)
	defer os.RemoveAll(_new)

	for _, _test := range _GITMATCHES {
		_path := filepath.Join(_new, _test.Local())
		_match := _ignore.Match(_path)
		if _match != nil {
			t.Fatalf("unexpected match; expected nil, got %v", _match)
		}
	}
} // TestMatchAbsolute()

func TestMatchRelative(t *testing.T) {
	// create a temporary .gitignore
	_buffer, _err := buffer(_GITMATCH)
	if _err != nil {
		t.Fatalf("unable to create temporary .gitignore: %s", _err.Error())
	}

	// ensure we can run New()
	//		- ensure we encounter no errors
	_position := []gitignore.Position{}
	_error := func(e gitignore.Error) bool {
		_position = append(_position, e.Position())
		return true
	}

	// ensure we have a non-nil GitIgnore instance
	_ignore := gitignore.New(_buffer, _GITBASE, _error)
	if _ignore == nil {
		t.Error("expected non-nil GitIgnore instance; nil found")
	}

	// ensure we encountered the right number of errors
	if len(_position) != _GITBADMATCHPATTERNS {
		t.Errorf(
			"match error mismatch; expected %d errors, got %d",
			_GITBADMATCHPATTERNS, len(_position),
		)
	}

	// perform the relative path matching
	_cb := func(path string, isdir bool) gitignore.Match {
		return _ignore.Relative(path, isdir)
	}
	for _, _test := range _GITMATCHES {
		do(t, _cb, _test)
	}
} // TestMatchRelative()

func do(t *testing.T, cb func(string, bool) gitignore.Match, m match) {
	// attempt to match this path
	_match := cb(m.Local(), m.IsDir())
	if _match == nil {
		// we have no match, is this expected?
		//		- a test that matches will list the expected pattern
		if m.Pattern != "" {
			t.Errorf(
				"failed match; expected match for %q by %q",
				m.Path, m.Pattern,
			)
			return
		}

		// since we have no match, ensure this path is not ignored
		if m.Ignore {
			t.Errorf(
				"failed ignore; no match for %q but expected to be ignored",
				m.Path,
			)
		}
	} else {
		// we have a match, is this expected?
		//		- a test that matches will list the expected pattern
		if m.Pattern == "" {
			t.Errorf(
				"unexpected match by %q; expected no match for %q",
				_match, m.Path,
			)
			return
		} else if m.Pattern != _match.String() {
			t.Errorf(
				"mismatch for %q; expected match pattern %q, got %q",
				m.Path, m.Pattern, _match.String(),
			)
			return
		}

		// since we have a match, are we expected to ignore this file?
		if m.Ignore != _match.Ignore() {
			t.Errorf(
				"ignore mismatch; expected %v for %q Ignore(), "+
					"got %v from pattern %q",
				m.Ignore, m.Path, _match.Ignore(), _match,
			)
		}
	}
} // do()

func directory(t *testing.T) (string, gitignore.GitIgnore) {
	// we need to populate a directory with the match test files
	//		- this is to permit GitIgnore.Match() to correctly resolve
	//		  absolute path names
	//		- populate the directory by passing a map of file names and their
	//		  contents
	//		- the content is not important, it just can't be empty
	//		- use this mechanism to also populate the .gitignore file
	_map := map[string]string{gitignore.File: _GITMATCH}
	for _, _test := range _GITMATCHES {
		_map[_test.Path] = " " // this is the file contents
	}

	// create the temporary directory
	_dir, _err := dir(_map)
	if _err != nil {
		t.Fatalf("unable to create temporary .gitignore: %s", _err.Error())
	}

	// ensure we can run New()
	//		- ensure we encounter no errors
	_position := []gitignore.Position{}
	_error := func(e gitignore.Error) bool {
		_position = append(_position, e.Position())
		return true
	}

	// ensure we have a non-nil GitIgnore instance
	_file := filepath.Join(_dir, gitignore.File)
	_ignore := gitignore.NewWithErrors(_file, _error)
	if _ignore == nil {
		t.Fatalf("expected non-nil GitIgnore instance; nil found")
	}

	// ensure we encountered the right number of errors
	if len(_position) != _GITBADMATCHPATTERNS {
		t.Errorf(
			"match error mismatch; expected %d errors, got %d",
			_GITBADMATCHPATTERNS, len(_position),
		)
	}

	// return the directory name and the GitIgnore instance
	return _dir, _ignore
} // directory()
