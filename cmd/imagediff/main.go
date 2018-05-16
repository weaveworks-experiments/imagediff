package main

import (
	"os"

	"github.com/weaveworks-experiments/imagediff/pkg/diff"
)

func main() {
	if len(os.Args) != 3 {
		panic("Please provide two Docker image tags to compare")
	}
	x := os.Args[1]
	y := os.Args[2]
	diff.Diff(x, y)
}
