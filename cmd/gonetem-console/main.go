package main

import (
	"github.com/mroy31/gonetem/internal/console"
	"github.com/mroy31/gonetem/internal/options"
)

func main() {
	options.InitConsoleConfig()
	console.Init()
	console.Execute()
}
