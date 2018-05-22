package main

import (
	"fmt"

	log "github.com/sirupsen/logrus"
	flag "github.com/spf13/pflag"
	"github.com/weaveworks-experiments/imagediff/pkg/diff"
	"github.com/weaveworks-experiments/imagediff/pkg/repository"
)

func main() {
	dockerConfigPath := flag.String("docker-config-path", "~/.docker/config.json", "Path to your Docker config.json file. This file contains your credentials to authenticate against private Docker registries.")
	sshPrivateKeyPath := flag.String("ssh-private-key-path", "~/.ssh/id_rsa", "Path to the private SSH key to use to authenticate against private Git repositories.")
	flag.Parse()
	args := flag.Args()
	if len(args) != 2 {
		log.Fatal("Please provide two Docker image tags to compare")
	}
	x := args[0]
	y := args[1]
	changeLog, err := diff.Diff(x, y, &diff.Options{
		DockerConfigPath: string(*dockerConfigPath),
		GitOptions: &repository.Options{
			SSHPrivateKeyPath: string(*sshPrivateKeyPath),
		},
	})
	if err != nil {
		log.WithFields(log.Fields{
			"x": x,
			"y": y,
		}).Fatal(err)
	}
	for _, change := range changeLog {
		fmt.Printf("%v %v\n", change.Revision[:7], change.Message)
	}
}
