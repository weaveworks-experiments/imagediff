package main

import (
	"github.com/weaveworks-experiments/imagediff/pkg/diff"

	flag "github.com/spf13/pflag"
)

func main() {
	flag.Parse()
	args := flag.Args()
	if len(args) != 2 {
		panic("Please provide two Docker image tags to compare")
	}
	x := args[0]
	y := args[1]
	diff.Diff(x, y)
}
