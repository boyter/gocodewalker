package gitignore_test

import (
	"testing"

	"os"
	"path/filepath"

	"github.com/denormal/go-gitignore"
)

type gitignoretest struct {
	errors   []gitignore.Error
	error    func(gitignore.Error) bool
	cache    gitignore.Cache
	cached   bool
	instance func(string) (gitignore.GitIgnore, error)
} // gitignoretest{}

func TestNewFromFile(t *testing.T) {
	_test := &gitignoretest{}
	_test.instance = func(file string) (gitignore.GitIgnore, error) {
		return gitignore.NewFromFile(file)
	}

	// perform the gitignore test
	withfile(t, _test, _GITIGNORE)
} // TestNewFromFile()

func TestNewFromWhitespaceFile(t *testing.T) {
	_test := &gitignoretest{}
	_test.instance = func(file string) (gitignore.GitIgnore, error) {
		return gitignore.NewFromFile(file)
	}

	// perform the gitignore test
	withfile(t, _test, _GITIGNORE_WHITESPACE)
} // TestNewFromWhitespaceFile()

func TestNewFromEmptyFile(t *testing.T) {
	_test := &gitignoretest{}
	_test.instance = func(file string) (gitignore.GitIgnore, error) {
		return gitignore.NewFromFile(file)
	}

	// perform the gitignore test
	withfile(t, _test, "")
} // TestNewFromEmptyFile()

func TestNewWithErrors(t *testing.T) {
	_test := &gitignoretest{}
	_test.error = func(e gitignore.Error) bool {
		_test.errors = append(_test.errors, e)
		return true
	}
	_test.instance = func(file string) (gitignore.GitIgnore, error) {
		// reset the error slice
		_test.errors = make([]gitignore.Error, 0)

		// attempt to create the GitIgnore instance
		_ignore := gitignore.NewWithErrors(file, _test.error)

		// if we encountered errors, and the first error has a zero position
		// then it represents a file access error
		//		- extract the error and return it
		//		- remove it from the list of errors
		var _err error
		if len(_test.errors) > 0 {
			if _test.errors[0].Position().Zero() {
				_err = _test.errors[0].Underlying()
				_test.errors = _test.errors[1:]
			}
		}

		// return the GitIgnore instance
		return _ignore, _err
	}

	// perform the gitignore test
	withfile(t, _test, _GITIGNORE)

	_test.error = nil
	withfile(t, _test, _GITIGNORE)
} // TestNewWithErrors()

func TestNewWithCache(t *testing.T) {
	// perform the gitignore test with a custom cache
	_test := &gitignoretest{}
	_test.cached = true
	_test.cache = gitignore.NewCache()
	_test.instance = func(file string) (gitignore.GitIgnore, error) {
		// reset the error slice
		_test.errors = make([]gitignore.Error, 0)

		// attempt to create the GitIgnore instance
		_ignore := gitignore.NewWithCache(file, _test.cache, _test.error)

		// if we encountered errors, and the first error has a zero position
		// then it represents a file access error
		//		- extract the error and return it
		//		- remove it from the list of errors
		var _err error
		if len(_test.errors) > 0 {
			if _test.errors[0].Position().Zero() {
				_err = _test.errors[0].Underlying()
				_test.errors = _test.errors[1:]
			}
		}

		// return the GitIgnore instance
		return _ignore, _err
	}

	// perform the gitignore test
	withfile(t, _test, _GITIGNORE)

	// repeat the tests while accumulating errors
	_test.error = func(e gitignore.Error) bool {
		_test.errors = append(_test.errors, e)
		return true
	}
	withfile(t, _test, _GITIGNORE)

	// create a temporary .gitignore
	_file, _err := file(_GITIGNORE)
	if _err != nil {
		t.Fatalf("unable to create temporary .gitignore: %s", _err.Error())
	}
	defer os.Remove(_file.Name())

	// attempt to load the .gitignore file
	_ignore, _err := _test.instance(_file.Name())
	if _err != nil {
		t.Fatalf("unable to open temporary .gitignore: %s", _err.Error())
	}

	// remove the .gitignore and try again
	os.Remove(_file.Name())

	// ensure the retrieved GitIgnore matches the stored instance
	_new, _err := _test.instance(_file.Name())
	if _err != nil {
		t.Fatalf(
			"unexpected error retrieving cached .gitignore: %s", _err.Error(),
		)
	} else if _new != _ignore {
		t.Fatalf(
			"gitignore.NewWithCache() mismatch; expected %v, got %v",
			_ignore, _new,
		)
	}
} // TestNewWithCache()

func TestNew(t *testing.T) {
	// create a temporary .gitignore
	_file, _err := file(_GITIGNORE)
	if _err != nil {
		t.Fatalf("unable to create temporary .gitignore: %s", _err.Error())
	}
	defer os.Remove(_file.Name())

	// ensure we can run NewGitIgnore()
	//		- ensure we encounter the expected errors
	_position := []gitignore.Position{}
	_error := func(e gitignore.Error) bool {
		_position = append(_position, e.Position())
		return true
	}

	_dir := filepath.Dir(_file.Name())
	_ignore := gitignore.New(_file, _dir, _error)

	// ensure we have a non-nil GitIgnore instance
	if _ignore == nil {
		t.Error("expected non-nil GitIgnore instance; nil found")
	}

	// ensure the base of the ignore is the directory of the temporary file
	if _ignore.Base() != _dir {
		t.Errorf(
			"gitignore.Base() mismatch; expected %q, got %q",
			_dir, _ignore.Base(),
		)
	}

	// ensure we encountered the right number of errors
	if len(_position) != _GITBADPATTERNS {
		t.Errorf(
			"parse error mismatch; expected %d errors, got %d",
			_GITBADPATTERNS, len(_position),
		)
	} else {
		// ensure the error positions are correct
		for _i := 0; _i < _GITBADPATTERNS; _i++ {
			_got := _position[_i]
			_expected := _GITBADPOSITION[_i]

			// ensure the positions are correct
			if !coincident(_got, _expected) {
				t.Errorf("bad pattern position mismatch; expected %q, got %q",
					pos(_expected), pos(_got),
				)
			}
		}
	}
} // TestNew()

func withfile(t *testing.T, test *gitignoretest, content string) {
	// create a temporary .gitignore
	_file, _err := file(content)
	if _err != nil {
		t.Fatalf("unable to create temporary .gitignore: %s", _err.Error())
	}
	defer os.Remove(_file.Name())

	// attempt to retrieve the GitIgnore instance
	_ignore, _err := test.instance(_file.Name())
	if _err != nil {
		t.Fatalf("unable to open temporary .gitignore: %s", _err.Error())
	}

	// ensure we have a non-nil GitIgnore instance
	if _ignore == nil {
		t.Error("expected non-nil GitIgnore instance; nil found")
	}

	// ensure the base of the ignore is the directory of the temporary file
	_dir := filepath.Dir(_file.Name())
	if _ignore.Base() != _dir {
		t.Errorf(
			"gitignore.Base() mismatch; expected %q, got %q",
			_dir, _ignore.Base(),
		)
	}

	// ensure we encountered the right number of errors
	//		- only do this if we are configured to record bad patterns
	if test.error != nil {
		if len(test.errors) != _GITBADPATTERNS {
			t.Errorf(
				"parse error mismatch; expected %d errors, got %d",
				_GITBADPATTERNS, len(test.errors),
			)
		} else {
			// ensure the error positions are correct
			for _i := 0; _i < _GITBADPATTERNS; _i++ {
				_got := test.errors[_i].Position()
				_expected := _GITBADPOSITION[_i]

				// augment the expected position with the test file name
				_expected.File = _file.Name()

				// ensure the positions are correct
				if !coincident(_got, _expected) {
					t.Errorf(
						"bad pattern position mismatch; expected %q, got %q",
						pos(_expected), pos(_got),
					)
				}
			}
		}
	}

	// test NewFromFile() behaves as expected if the .gitgnore file does
	// not exist
	_err = os.Remove(_file.Name())
	if _err != nil {
		t.Fatalf(
			"unable to remove temporary .gitignore %s: %s",
			_file.Name(), _err.Error(),
		)
	}
	_ignore, _err = test.instance(_file.Name())
	if _err == nil {
		// if we are using a cache in this test, then no error is acceptable
		// as long as a GitIgnore instance is retrieved
		if test.cached {
			if _ignore == nil {
				t.Fatal("expected non-nil GitIgnore, nil found")
			}
		} else if test.error != nil {
			t.Fatalf(
				"expected error loading deleted file %s; none found",
				_file.Name(),
			)
		}
	} else if !os.IsNotExist(_err) {
		t.Fatalf(
			"unexpected error attempting to load non-existant .gitignore: %s",
			_err.Error(),
		)
	} else if _ignore != nil {
		t.Fatalf("expected nil GitIgnore, got %v", _ignore)
	}

	// test NewFromFile() behaves as expected if absolute path of the
	// .gitignore cannot be determined
	_map := map[string]string{gitignore.File: content}
	_dir, _err = dir(_map)
	if _err != nil {
		t.Fatalf("unable to create temporary directory: %s", _err.Error())
	}
	defer os.RemoveAll(_dir)

	// change into the temporary directory
	_cwd, _err := os.Getwd()
	if _err != nil {
		t.Fatalf("unable to retrieve working directory: %s", _err.Error())
	}
	_err = os.Chdir(_dir)
	if _err != nil {
		t.Fatalf("unable to chdir into temporary directory: %s", _err.Error())
	}
	defer os.Chdir(_cwd)

	// remove permission from the temporary directory
	_err = os.Chmod(_dir, 0)
	if _err != nil {
		t.Fatalf(
			"unable to remove temporary directory permissions: %s: %s",
			_dir, _err.Error(),
		)
	}

	// attempt to load the .gitignore using a relative path
	_ignore, _err = test.instance(gitignore.File)
	if test.error != nil && _err == nil {
		_git := filepath.Join(_dir, gitignore.File)
		t.Fatalf(
			"%s: expected error for inaccessible .gitignore; none found",
			_git,
		)
	} else if _ignore != nil {
		t.Fatalf("expected nil GitIgnore, got %v", _ignore)
	}
} // withfile()
