package main

import (
	"fmt"
	"os"

	"github.com/git-rain/git-rain/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
