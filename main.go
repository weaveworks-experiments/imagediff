package main

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/docker/docker/pkg/term"
	"github.com/src-d/go-git/storage/memory"

	git "gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
)

func main() {
	if len(os.Args) != 3 {
		panic("Please provide two Docker image tags to compare")
	}
	x := os.Args[1]
	y := os.Args[2]
	docker, err := client.NewEnvClient()
	if err != nil {
		panic(err)
	}
	pull(docker, x)
	pull(docker, y)
	xLabels := imageLabels(docker, x)
	yLabels := imageLabels(docker, y)
	xVcsURL, xVcsRef := vcsURLAndRef(xLabels)
	yVcsURL, yVcsRef := vcsURLAndRef(yLabels)
	validate(x, y, xVcsURL, yVcsURL, xVcsRef, yVcsRef)
	r := gitClone(xVcsURL)
	xCommit := commit(r, xVcsRef)
	yCommit := commit(r, yVcsRef)
	printChangeLog(xCommit, yCommit)
}

func pull(docker *client.Client, imageName string) {
	resp, err := docker.ImagePull(context.Background(), imageName, types.ImagePullOptions{})
	if err != nil {
		panic(err)
	}
	defer resp.Close()
	fd, isTerminal := term.GetFdInfo(ioutil.Discard)
	if err := jsonmessage.DisplayJSONMessagesStream(resp, ioutil.Discard, fd, isTerminal, nil); err != nil {
		panic(err)
	}
}

func imageLabels(docker *client.Client, imageName string) map[string]string {
	args := filters.NewArgs()
	args.Add("reference", imageName)
	images, err := docker.ImageList(context.Background(), types.ImageListOptions{
		Filters: args,
	})
	if err != nil {
		panic(err)
	}
	for _, image := range images {
		return image.Labels
	}
	panic("Image not found: " + imageName)
}

func vcsURLAndRef(labels map[string]string) (string, string) {
	vcsURL := ""
	vcsRef := ""
	for label, value := range labels {
		switch label {
		case "org.opencontainers.image.source":
			vcsURL = value
		case "org.label-schema.vcs-url":
			vcsURL = value
		case "org.opencontainers.image.revision":
			vcsRef = value
		case "org.label-schema.vcs-ref":
			vcsRef = value
		}
	}
	return vcsURL, vcsRef
}

func validate(x, y, xVcsURL, yVcsURL, xVcsRef, yVcsRef string) {
	if xVcsURL == "" {
		panic("No repository for " + x)
	}
	if yVcsURL == "" {
		panic("No repository for " + y)
	}
	if xVcsRef == "" {
		panic("No commit hash for " + x)
	}
	if yVcsRef == "" {
		panic("No commit hash for " + y)
	}
	if xVcsURL != yVcsURL {
		panic("Source code in different repositories")
	}
}

func gitClone(vcsURL string) *git.Repository {
	storage := memory.NewStorage()
	r, err := git.Clone(storage, nil, &git.CloneOptions{
		URL: vcsURL,
	})
	if err != nil {
		panic(err)
	}
	return r
}

var errFound = errors.New("<Found>")

// Workaround to resolve a short hash to a full commit object.
// Once the following PR is merged, we should be able to do this in a more elegant way.
// See also: https://github.com/src-d/go-git/pull/706
func commit(r *git.Repository, shortHash string) *object.Commit {
	head, err := r.Head()
	if err != nil {
		panic(err)
	}
	commitsIter, err := r.Log(&git.LogOptions{From: head.Hash()})
	if err != nil {
		panic(err)
	}
	var commit *object.Commit
	err = commitsIter.ForEach(func(c *object.Commit) error {
		if strings.HasPrefix(c.Hash.String(), shortHash) {
			commit = c
			return errFound
		}
		return nil
	})
	if err == nil || err != errFound {
		panic(fmt.Sprintf("Commit with short hash %s could not be found: %s", shortHash, err))
	}
	return commit
}

func printChangeLog(xCommit, yCommit *object.Commit) {
	err := object.NewCommitPostorderIter(yCommit, nil).ForEach(func(c *object.Commit) error {
		if c.Hash == xCommit.Hash {
			return errFound
		}
		fmt.Printf("%s %s\n", c.Hash.String()[:7], strings.Split(c.Message, "\n")[0])
		return nil
	})
	if err == nil || err != errFound {
		panic(fmt.Sprintf("Commit with hash %s could not be found: %s", xCommit.Hash, err))
	}
}
