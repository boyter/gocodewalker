package gitignore_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/denormal/go-gitignore"
)

type repositorytest struct {
	file      string
	directory string
	cache     gitignore.Cache
	cached    bool
	error     func(e gitignore.Error) bool
	errors    []gitignore.Error
	bad       int
	instance  func(string) (gitignore.GitIgnore, error)
	exclude   string
	gitdir    string
} // repostorytest{}

func (r *repositorytest) create(path string, gitdir bool) (gitignore.GitIgnore, error) {
	// if we have an error handler, reset the list of errors
	if r.error != nil {
		r.errors = make([]gitignore.Error, 0)
	}

	if r.file == gitignore.File || r.file == "" {
		// should we create the global exclude file
		r.gitdir = os.Getenv("GIT_DIR")
		if gitdir {
			// create a temporary file for the global exclude file
			_exclude, _err := exclude(_GITEXCLUDE)
			if _err != nil {
				return nil, _err
			}

			// extract the current value of the GIT_DIR environment variable
			// and set the value to be that of the temporary file
			r.exclude = _exclude
			_err = os.Setenv("GIT_DIR", r.exclude)
			if _err != nil {
				return nil, _err
			}
		} else {
			_err := os.Unsetenv("GIT_DIR")
			if _err != nil {
				return nil, _err
			}
		}
	}

	// attempt to create the GitIgnore instance
	_repository, _err := r.instance(path)

	// if we encountered errors, and the first error has a zero position
	// then it represents a file access error
	//		- extract the error and return it
	//		- remove it from the list of errors
	if len(r.errors) > 0 {
		if r.errors[0].Position().Zero() {
			_err = r.errors[0].Underlying()
			r.errors = r.errors[1:]
		}
	}

	// return the GitIgnore instance
	return _repository, _err
} // create()

func (r *repositorytest) destroy() {
	// remove the temporary files and directories
	for _, _path := range []string{r.directory, r.exclude} {
		if _path != "" {
			defer os.RemoveAll(_path)
		}
	}

	if r.file == gitignore.File || r.file == "" {
		// reset the GIT_DIR environment variable
		if r.gitdir == "" {
			defer os.Unsetenv("GIT_DIR")
		} else {
			defer os.Setenv("GIT_DIR", r.gitdir)
		}
	}
} // destroy()

type invalidtest struct {
	*repositorytest
	tag   string
	match func() gitignore.Match
} // invalidtest{}

func TestRepository(t *testing.T) {
	_test := &repositorytest{}
	_test.bad = _GITREPOSITORYERRORS
	_test.instance = func(path string) (gitignore.GitIgnore, error) {
		return gitignore.NewRepository(path)
	}

	// perform the repository tests
	repository(t, _test, _REPOSITORYMATCHES)

	// remove the temporary directory used for this test
	defer _test.destroy()
} // TestRepository()

func TestRepositoryWithFile(t *testing.T) {
	_test := &repositorytest{}
	_test.bad = _GITREPOSITORYERRORS
	_test.file = gitignore.File + "-with-file"
	_test.instance = func(path string) (gitignore.GitIgnore, error) {
		return gitignore.NewRepositoryWithFile(path, _test.file)
	}

	// perform the repository tests
	repository(t, _test, _REPOSITORYMATCHES)

	// remove the temporary directory used for this test
	defer _test.destroy()
} // TestRepositoryWithFile()

func TestRepositoryWithErrors(t *testing.T) {
	_test := &repositorytest{}
	_test.bad = _GITREPOSITORYERRORS
	_test.file = gitignore.File + "-with-errors"
	_test.error = func(e gitignore.Error) bool {
		_test.errors = append(_test.errors, e)
		return true
	}
	_test.instance = func(path string) (gitignore.GitIgnore, error) {
		return gitignore.NewRepositoryWithErrors(
			path, _test.file, _test.error,
		), nil
	}

	// perform the repository tests
	repository(t, _test, _REPOSITORYMATCHES)

	// remove the temporary directory used for this test
	defer _test.destroy()
} // TestRepositoryWithErrors()

func TestRepositoryWithErrorsFalse(t *testing.T) {
	_test := &repositorytest{}
	_test.bad = _GITREPOSITORYERRORSFALSE
	_test.file = gitignore.File + "-with-errors-false"
	_test.error = func(e gitignore.Error) bool {
		_test.errors = append(_test.errors, e)
		return false
	}
	_test.instance = func(path string) (gitignore.GitIgnore, error) {
		return gitignore.NewRepositoryWithErrors(
			path, _test.file, _test.error,
		), nil
	}

	// perform the repository tests
	repository(t, _test, _REPOSITORYMATCHESFALSE)

	// remove the temporary directory used for this test
	defer _test.destroy()
} // TestRepositoryWithErrorsFalse()

func TestRepositoryWithCache(t *testing.T) {
	_test := &repositorytest{}
	_test.bad = _GITREPOSITORYERRORS
	_test.cache = gitignore.NewCache()
	_test.cached = true
	_test.instance = func(path string) (gitignore.GitIgnore, error) {
		return gitignore.NewRepositoryWithCache(
			path, _test.file, _test.cache, _test.error,
		), nil
	}

	// perform the repository tests
	repository(t, _test, _REPOSITORYMATCHES)

	// clean up
	defer _test.destroy()

	// rerun the tests while accumulating errors
	_test.directory = ""
	_test.file = gitignore.File + "-with-cache"
	_test.error = func(e gitignore.Error) bool {
		_test.errors = append(_test.errors, e)
		return true
	}
	repository(t, _test, _REPOSITORYMATCHES)

	// remove the temporary directory used for this test
	_err := os.RemoveAll(_test.directory)
	if _err != nil {
		t.Fatalf(
			"unable to remove temporary directory %s: %s",
			_test.directory, _err.Error(),
		)
	}

	// recreate the temporary directory
	//		- this remove & recreate gives us an empty directory for the
	//		  repository test
	//		- this lets us test the caching
	_err = os.MkdirAll(_test.directory, _GITMASK)
	if _err != nil {
		t.Fatalf(
			"unable to recreate temporary directory %s: %s",
			_test.directory, _err.Error(),
		)
	}
	defer _test.destroy()

	// repeat the repository tests
	//		- these should succeed using just the cache data
	repository(t, _test, _REPOSITORYMATCHES)
} // TestRepositoryWithCache()

func TestInvalidRepository(t *testing.T) {
	_test := &repositorytest{}
	_test.instance = func(path string) (gitignore.GitIgnore, error) {
		return gitignore.NewRepository(path)
	}

	// perform the invalid repository tests
	invalid(t, _test)
} // TestInvalidRepository()

func TestInvalidRepositoryWithFile(t *testing.T) {
	_test := &repositorytest{}
	_test.file = gitignore.File + "-invalid-with-file"
	_test.instance = func(path string) (gitignore.GitIgnore, error) {
		return gitignore.NewRepositoryWithFile(path, _test.file)
	}

	// perform the invalid repository tests
	invalid(t, _test)
} // TestInvalidRepositoryWithFile()

func TestInvalidRepositoryWithErrors(t *testing.T) {
	_test := &repositorytest{}
	_test.file = gitignore.File + "-invalid-with-errors"
	_test.error = func(e gitignore.Error) bool {
		_test.errors = append(_test.errors, e)
		return true
	}
	_test.instance = func(path string) (gitignore.GitIgnore, error) {
		return gitignore.NewRepositoryWithErrors(
			path, _test.file, _test.error,
		), nil
	}

	// perform the invalid repository tests
	invalid(t, _test)
} // TestInvalidRepositoryWithErrors()

func TestInvalidRepositoryWithErrorsFalse(t *testing.T) {
	_test := &repositorytest{}
	_test.file = gitignore.File + "-invalid-with-errors-false"
	_test.error = func(e gitignore.Error) bool {
		_test.errors = append(_test.errors, e)
		return false
	}
	_test.instance = func(path string) (gitignore.GitIgnore, error) {
		return gitignore.NewRepositoryWithErrors(
			path, _test.file, _test.error,
		), nil
	}

	// perform the invalid repository tests
	invalid(t, _test)
} // TestInvalidRepositoryWithErrorsFalse()

func TestInvalidRepositoryWithCache(t *testing.T) {
	_test := &repositorytest{}
	_test.file = gitignore.File + "-invalid-with-cache"
	_test.cache = gitignore.NewCache()
	_test.cached = true
	_test.error = func(e gitignore.Error) bool {
		_test.errors = append(_test.errors, e)
		return true
	}
	_test.instance = func(path string) (gitignore.GitIgnore, error) {
		return gitignore.NewRepositoryWithCache(
			path, _test.file, _test.cache, _test.error,
		), nil
	}

	// perform the invalid repository tests
	invalid(t, _test)

	// repeat the tests using a default cache
	_test.cache = nil
	invalid(t, _test)
} // TestInvalidRepositoryWithCache()

//
// helper functions
//

func repository(t *testing.T, test *repositorytest, m []match) {
	// if the test has no configured directory, then create a new
	// directory with the required .gitignore files
	if test.directory == "" {
		// what name should we use for the .gitignore file?
		//		- if none is given, use the default
		_file := test.file
		if _file == "" {
			_file = gitignore.File
		}

		// create a temporary directory populated with sample .gitignore files
		//		- first, augment the test data to include file names
		_map := make(map[string]string)
		for _k, _content := range _GITREPOSITORY {
			_map[_k+"/"+_file] = _content
		}
		_dir, _err := dir(_map)
		if _err != nil {
			t.Fatalf("unable to create temporary directory: %s", _err.Error())
		}
		test.directory = _dir
	}

	// create the repository
	_repository, _err := test.create(test.directory, true)
	if _err != nil {
		t.Fatalf("unable to create repository: %s", _err.Error())
	}

	// ensure we have a non-nill repository returned
	if _repository == nil {
		t.Error("expected non-nill GitIgnore repository instance; nil found")
	}

	// ensure the base of the repository is correct
	if _repository.Base() != test.directory {
		t.Errorf(
			"repository.Base() mismatch; expected %q, got %q",
			test.directory, _repository.Base(),
		)
	}

	// we need to check each test to see if it's matching against a
	// GIT_DIR/info/exclude
	//		- we only do this if the target does not use .gitignore
	//		  as the name of the ignore file
	_prepare := func(m match) match {
		if test.file == "" || test.file == gitignore.File {
			return m
		} else if m.Exclude {
			return match{m.Path, "", false, m.Exclude}
		} else {
			return m
		}
	} // _prepare()

	// perform the repository matching using absolute paths
	_cb := func(path string, isdir bool) gitignore.Match {
		_path := filepath.Join(_repository.Base(), path)
		return _repository.Absolute(_path, isdir)
	}
	for _, _test := range m {
		do(t, _cb, _prepare(_test))
	}

	// repeat the tests using relative paths
	_repository, _err = test.create(test.directory, true)
	if _err != nil {
		t.Fatalf("unable to create repository: %s", _err.Error())
	}
	_cb = func(path string, isdir bool) gitignore.Match {
		return _repository.Relative(path, isdir)
	}
	for _, _test := range m {
		do(t, _cb, _prepare(_test))
	}

	// perform absolute path tests with paths not under the same repository
	_map := make(map[string]string)
	for _, _test := range m {
		_map[_test.Path] = " "
	}
	_new, _err := dir(_map)
	if _err != nil {
		t.Fatalf("unable to create temporary directory: %s", _err.Error())
	}
	defer os.RemoveAll(_new)

	// first, perform Match() tests
	_repository, _err = test.create(test.directory, true)
	if _err != nil {
		t.Fatalf("unable to create repository: %s", _err.Error())
	}
	for _, _test := range m {
		_path := filepath.Join(_new, _test.Local())
		_match := _repository.Match(_path)
		if _match != nil {
			t.Fatalf("unexpected match; expected nil, got %v", _match)
		}
	}

	// next, perform Absolute() tests
	_repository, _err = test.create(test.directory, true)
	if _err != nil {
		t.Fatalf("unable to create repository: %s", _err.Error())
	}
	for _, _test := range m {
		// build the absolute path
		_path := filepath.Join(_new, _test.Local())

		// we don't expect to match paths not under this repository
		_match := _repository.Absolute(_path, _test.IsDir())
		if _match != nil {
			t.Fatalf("unexpected match; expected nil, got %v", _match)
		}
	}

	// now, repeat the Match() test after having first removed the
	// temporary directory
	//		- we are testing correct handling of missing files
	_err = os.RemoveAll(_new)
	if _err != nil {
		t.Fatalf(
			"unable to remove temporary directory %s: %s",
			_new, _err.Error(),
		)
	}
	_repository, _err = test.create(test.directory, true)
	if _err != nil {
		t.Fatalf("unable to create repository: %s", _err.Error())
	}
	for _, _test := range m {
		_path := filepath.Join(_new, _test.Local())

		// if we have an error handler configured, we should be recording
		// and error in this call to Match()
		_before := len(test.errors)

		// perform the match
		_match := _repository.Match(_path)
		if _match != nil {
			t.Fatalf("unexpected match; expected nil, got %v", _match)
		}

		// were we recording errors?
		if test.error != nil {
			_after := len(test.errors)
			if !(_after > _before) {
				t.Fatalf(
					"expected Match() error; none found for %s",
					_path,
				)
			}

			// ensure the most recent error is "not exists"
			_latest := test.errors[_after-1]
			_underlying := _latest.Underlying()
			if !os.IsNotExist(_underlying) {
				t.Fatalf(
					"unexpected Match() error for %s; expected %q, got %q",
					_path, os.ErrNotExist.Error(), _underlying.Error(),
				)
			}
		}
	}

	// ensure Match() behaves as expected if the absolute path cannot
	// be determined
	//		- we do this by choosing as our working directory a path
	//		  that this process does not have permission to
	_dir, _err := dir(nil)
	if _err != nil {
		t.Fatalf("unable to create temporary directory: %s", _err.Error())
	}
	defer os.RemoveAll(_dir)

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
			"unable to remove temporary directory %s: %s",
			_dir, _err.Error(),
		)
	}

	// perform the repository tests
	_repository, _err = test.create(test.directory, true)
	if _err != nil {
		t.Fatalf("unable to create repository: %s", _err.Error())
	}
	for _, _test := range m {
		_match := _repository.Match(_test.Local())
		if _match != nil {
			t.Fatalf("unexpected match; expected nil, not %v", _match)
		}
	}

	if test.errors != nil {
		// ensure the number of errors is expected
		if len(test.errors) != test.bad {
			t.Fatalf(
				"unexpected repository errors; expected %d, got %d",
				test.bad, len(test.errors),
			)
		} else {
			// if we're here, then we intended to record errors
			//		- ensure we recorded the expected errors
			for _i := 0; _i < len(test.errors); _i++ {
				_got := test.errors[_i]
				_underlying := _got.Underlying()
				if os.IsNotExist(_underlying) ||
					os.IsPermission(_underlying) {
					continue
				} else {
					t.Log(_i)
					t.Fatalf("unexpected repository error: %s", _got.Error())
				}
			}
		}
	}
} // repository()

func invalid(t *testing.T, test *repositorytest) {
	// create a temporary file to use as the repository
	_file, _err := file("")
	if _err != nil {
		t.Fatalf("unable to create temporary file: %s", _err.Error())
	}
	defer os.Remove(_file.Name())

	// test repository instance creation against a file
	_repository, _err := test.create(_file.Name(), false)
	if _err == nil {
		t.Errorf(
			"invalid repository error; expected %q, got nil",
			gitignore.InvalidDirectoryError.Error(),
		)
	} else if _err != gitignore.InvalidDirectoryError {
		t.Errorf(
			"invalid repository mismatch; expected %q, got %q",
			gitignore.InvalidDirectoryError.Error(), _err.Error(),
		)
	}

	// ensure no repository is returned
	if _repository != nil {
		t.Errorf(
			"invalid repository; expected nil, got %v",
			_repository,
		)
	}

	// now, remove the temporary file and repeat the tests
	_err = os.Remove(_file.Name())
	if _err != nil {
		t.Fatalf(
			"unable to remove temporary file %s: %s",
			_file.Name(), _err.Error(),
		)
	}

	// test repository instance creating against a missing file
	_repository, _err = test.create(_file.Name(), false)
	if _err == nil {
		t.Errorf(
			"invalid repository error; expected %q, got nil",
			gitignore.InvalidDirectoryError.Error(),
		)
	} else if !os.IsNotExist(_err) {
		t.Errorf(
			"invalid repository mismatch; "+
				"expected no such file or directory, got %q",
			_err.Error(),
		)
	}

	// ensure no repository is returned
	if _repository != nil {
		t.Errorf(
			"invalid repository; expected nil, got %v",
			_repository,
		)
	}

	// ensure we can't create a repository instance where the absolute path
	// of the repository cannot be determined
	//		- we do this by choosing a working directory this process does
	//		  not have access to and using a relative path
	_map := map[string]string{gitignore.File: _GITIGNORE}
	_dir, _err := dir(_map)
	if _err != nil {
		t.Fatalf("unable to create a temporary directory: %s", _err.Error())
	}
	defer os.RemoveAll(_dir)

	// now change the working directory
	_cwd, _err := os.Getwd()
	if _err != nil {
		t.Fatalf("unable to retrieve working directory: %s", _err.Error())
	}
	_err = os.Chdir(_dir)
	if _err != nil {
		t.Fatalf("unable to chdir into temporary directory: %s", _err.Error())
	}
	defer os.Chdir(_cwd)

	// remove permissions from the working directory
	_err = os.Chmod(_dir, 0)
	if _err != nil {
		t.Fatalf("unable remove temporary directory permissions: %s: %s",
			_dir, _err.Error(),
		)
	}

	// test repository instance creating against a relative path
	//		- the relative path exists
	_repository, _err = test.create(gitignore.File, false)
	if _err == nil {
		t.Errorf("expected repository error, got nil")
	} else if os.IsNotExist(_err) {
		t.Errorf(
			"unexpected repository error; file exists, but %q returned",
			_err.Error(),
		)
	}

	// next, create a repository where we do not have read permission
	// to a .gitignore file within the repository
	//		- this should trigger a panic() when attempting a file match
	for _, _test := range _REPOSITORYMATCHES {
		_map[_test.Path] = " "
	}
	_dir, _err = dir(_map)
	if _err != nil {
		t.Fatalf("unable to create a temporary directory: %s", _err.Error())
	}
	defer os.RemoveAll(_dir)

	_git := filepath.Join(_dir, gitignore.File)
	_err = os.Chmod(_git, 0)
	if _err != nil {
		t.Fatalf("unable remove temporary .gitignore permissions: %s: %s",
			_git, _err.Error(),
		)
	}

	// attempt to match a path in this repository
	//		- it can be anything, so we just use the .gitignore itself
	//		- between each test we recreate the repository instance to
	//		  remove the effect of any caching
	_instance := func() gitignore.GitIgnore {
		// reset the cache
		if test.cached {
			if test.cache != nil {
				test.cache = gitignore.NewCache()
			}
		}

		// create the new repository
		_repository, _err := test.create(_dir, false)
		if _err != nil {
			t.Fatalf("unable to create repository: %s", _err.Error())
		}

		// return the repository
		return _repository
	}
	for _, _match := range _REPOSITORYMATCHES {
		_local := _match.Local()
		_isdir := _match.IsDir()
		_path := filepath.Join(_dir, _local)

		// try Match() with an absolute path
		_test := &invalidtest{repositorytest: test}
		_test.tag = "Match()"
		_test.match = func() gitignore.Match {
			return _instance().Match(_path)
		}
		run(t, _test)

		// try Absolute() with an absolute path
		_test = &invalidtest{repositorytest: test}
		_test.tag = "Absolute()"
		_test.match = func() gitignore.Match {
			return _instance().Absolute(_path, _isdir)
		}
		run(t, _test)

		// try Absolute() with an absolute path
		_test = &invalidtest{repositorytest: test}
		_test.tag = "Relative()"
		_test.match = func() gitignore.Match {
			return _instance().Relative(_local, _isdir)
		}
		run(t, _test)
	}
} // invalid()

func run(t *testing.T, test *invalidtest) {
	// perform the match, and ensure it returns nil, nil
	_match := test.match()
	if _match != nil {
		t.Fatalf("%s: unexpected match: %v", test.tag, _match)
	} else if test.errors == nil {
		return
	}

	// if we're here, then we intended to record errors
	//		- ensure we recorded the expected errors
	for _i := 0; _i < len(test.errors); _i++ {
		_got := test.errors[_i]
		_underlying := _got.Underlying()
		if os.IsNotExist(_underlying) ||
			os.IsPermission(_underlying) {
			continue
		} else {
			t.Fatalf(
				"%s: unexpected error: %q",
				test.tag, _got.Error(),
			)
		}
	}
} // run()
