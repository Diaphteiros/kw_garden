package main

import (
	"os"

	"github.com/Diaphteiros/kw_garden/cmd"
)

func main() {
	err := cmd.RootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}
