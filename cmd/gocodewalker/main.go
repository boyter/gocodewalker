package main

import (
	"fmt"
	"github.com/boyter/gocodewalker"
	"os"
)

// designed to work against https://github.com/svent/gitignore-test
func main() {
	fileListQueue := make(chan *gocodewalker.File, 10_000)
	fileWalker := gocodewalker.NewFileWalker("/Users/boyter/Documents/projects/gitignore-test/", fileListQueue)

	go func() {
		err := fileWalker.Start()
		if err != nil {
			fmt.Println("ERR", err.Error())
		}
	}()

	for f := range fileListQueue {
		contents, _ := os.ReadFile(f.Location)
		if len(contents) > 10 {
			contents = contents[:10]
		}
		fmt.Print(fmt.Sprintf("%v:%v", f.Location, string(contents)))
	}
}
