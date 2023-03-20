package gitignore_test

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/denormal/go-gitignore"
)

func file(content string) (*os.File, error) {
	// create a temporary file
	_file, _err := ioutil.TempFile("", "gitignore")
	if _err != nil {
		return nil, _err
	}

	// populate this file with the example .gitignore
	_, _err = _file.WriteString(content)
	if _err != nil {
		defer os.Remove(_file.Name())
		return nil, _err
	}
	_, _err = _file.Seek(0, io.SeekStart)
	if _err != nil {
		defer os.Remove(_file.Name())
		return nil, _err
	}

	// we have a temporary file containing the .gitignore
	return _file, nil
} // file()

func dir(content map[string]string) (string, error) {
	// create a temporary directory
	_dir, _err := ioutil.TempDir("", "")
	if _err != nil {
		return "", _err
	}

	// resolve the path of this directory
	//		- we do this to handle systems with a temporary directory
	//		  that is a symbolic link
	_dir, _err = filepath.EvalSymlinks(_dir)
	if _err != nil {
		defer os.RemoveAll(_dir)
		return "", _err
	}

	// populate the temporary directory with the content map
	//		- each key of the map is a file name
	//		- each value of the map is the file content
	//		- file names are relative to the temporary directory
	if content != nil {
		for _key, _content := range content {
			// ensure we have content to store
			if _content == "" {
				continue
			}

			// should we create a directory or a file?
			_isdir := false
			_path := _key
			if strings.HasSuffix(_path, "/") {
				_path = strings.TrimSuffix(_path, "/")
				_isdir = true
			}

			// construct the absolute path (according to the local file system)
			_abs := _dir
			_parts := strings.Split(_path, "/")
			_last := len(_parts) - 1
			if _isdir {
				_abs = filepath.Join(_abs, filepath.Join(_parts...))
			} else if _last > 0 {
				_abs = filepath.Join(_abs, filepath.Join(_parts[:_last]...))
			}

			// ensure this directory exists
			_err = os.MkdirAll(_abs, _GITMASK)
			if _err != nil {
				defer os.RemoveAll(_dir)
				return "", _err
			} else if _isdir {
				continue
			}

			// create the absolute path for the target file
			_abs = filepath.Join(_abs, _parts[_last])

			// write the contents to this file
			_file, _err := os.Create(_abs)
			if _err != nil {
				defer os.RemoveAll(_dir)
				return "", _err
			}
			_, _err = _file.WriteString(_content)
			if _err != nil {
				defer os.RemoveAll(_dir)
				return "", _err
			}
			_err = _file.Close()
			if _err != nil {
				defer os.RemoveAll(_dir)
				return "", _err
			}
		}
	}

	// return the temporary directory name
	return _dir, nil
} // dir()

func exclude(content string) (string, error) {
	// create a temporary folder with the info/ subfolder
	_dir, _err := dir(nil)
	if _err != nil {
		return "", _err
	}
	_info := filepath.Join(_dir, "info")
	_err = os.MkdirAll(_info, _GITMASK)
	if _err != nil {
		defer os.RemoveAll(_dir)
		return "", _err
	}

	// create the exclude file
	_exclude := filepath.Join(_info, "exclude")
	_err = ioutil.WriteFile(_exclude, []byte(content), _GITMASK)
	if _err != nil {
		defer os.RemoveAll(_dir)
		return "", _err
	}

	// return the temporary directory name
	return _dir, nil
} // exclude()

func coincident(a, b gitignore.Position) bool {
	return a.File == b.File &&
		a.Line == b.Line &&
		a.Column == b.Column &&
		a.Offset == b.Offset
} // coincident()

func pos(p gitignore.Position) string {
	_prefix := p.File
	if _prefix != "" {
		_prefix = _prefix + ": "
	}

	return fmt.Sprintf("%s%d:%d [%d]", _prefix, p.Line, p.Column, p.Offset)
} // pos()

func buffer(content string) (*bytes.Buffer, error) {
	// return a buffered .gitignore
	return bytes.NewBufferString(content), nil
} // buffer()

func null() gitignore.GitIgnore {
	// return an empty GitIgnore instance
	return gitignore.New(bytes.NewBuffer(nil), "", nil)
} // null()
