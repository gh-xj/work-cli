package main

import (
	"os"

	"github.com/gh-xj/work-cli/internal/workcli"
)

func main() {
	os.Exit(workcli.Execute(os.Args[1:]))
}
