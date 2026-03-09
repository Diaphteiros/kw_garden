package main

import (
	"os"

	"github.com/spf13/cobra/doc"
	"github.com/Diaphteiros/kw_garden/cmd"
)

func main() {
	if len(os.Args) < 2 {
		panic("documentation folder path required as argument")
	}
	if err := doc.GenMarkdownTree(cmd.RootCmd, os.Args[1]); err != nil {
		panic(err)
	}
}
