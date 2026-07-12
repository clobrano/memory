package main

import (
	"os"

	"github.com/clobrano/memory/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
