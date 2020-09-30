# go-code-walker

[![Go Report Card](https://goreportcard.com/badge/github.com/boyter/go-code-walker)](https://goreportcard.com/report/github.com/boyter/go-code-walker)
[![Str Count Badge](https://sloc.xyz/github/boyter/go-code-walker/)](https://github.com/boyter/go-code-walker/)

Library to help with walking of code directories in Go

https://pkg.go.dev/github.com/boyter/go-code-walker

Package provides file operations specific to code repositories such as walking the file tree obeying .ignore and .gitignore files
or looking for the root directory assuming already in a git project.

Note that it currently has a dependancy on go-gitignore which is pulled in here to avoid external dependencies. This needs to be rewritten
as there are some bugs in that implementation.

Example of usage,

```
fileListQueue := make(chan *file.File, 100)

fileWalker := file.NewFileWalker(".", fileListQueue)
fileWalker.AllowListExtensions = append(fileWalker.AllowListExtensions, "go")

go fileWalker.Start()

for f := range fileListQueue {
    fmt.Println(f.Location)
}
```

The above by default will recursively add files to the fileListQueue respecting both .ignore and .gitignore files if found, and
only adding files with the go extension into the queue.

All code is dual-licenced as either MIT or Unlicence.
Note that as an Australian I cannot put this into the public domain, hence the choice most liberal licences I can find.
