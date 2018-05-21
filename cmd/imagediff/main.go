package main

import (
	"github.com/weaveworks-experiments/imagediff/pkg/diff"

	flag "github.com/spf13/pflag"
)

func main() {
	dockerConfigPath := flag.String("docker-config-path", "~/.docker/config.json", "Path to your Docker config.json file. This file contains your credentials to authenticate against private Docker registries.")
	flag.Parse()
	args := flag.Args()
	if len(args) != 2 {
		panic("Please provide two Docker image tags to compare")
	}
	x := args[0]
	y := args[1]
	diff.Diff(x, y, diff.Options{
		DockerConfigPath: string(*dockerConfigPath),
	})
}
