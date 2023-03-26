// SPDX-License-Identifier: MIT OR Unlicense

package main

import (
	"fmt"
	"github.com/boyter/gocodewalker"
	"regexp"
)

func main() {
	fileListQueue := make(chan *gocodewalker.File, 100)
	fileWalker := gocodewalker.NewFileWalker(".", fileListQueue)

	fileWalker.AllowListExtensions = []string{"go", "sh"}
	fileWalker.ExcludeListExtensions = []string{"sh"}
	fileWalker.IncludeFilenameRegex = []*regexp.Regexp{regexp.MustCompile(".*")}

	// handle the error by printing it out and terminating the walker and returning
	// false which should cause continued processing to error
	errorHandler := func(e error) bool {
		fmt.Println("ERR", e.Error())
		fileWalker.Terminate()
		return false
	}
	fileWalker.SetErrorHandler(errorHandler)

	go func() {
		err := fileWalker.Start()
		if err != nil {
			fmt.Println("ERROR", err.Error())
		}
	}()

	for f := range fileListQueue {
		fmt.Println(f.Location)
	}
}
