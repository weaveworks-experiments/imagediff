package diff

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/weaveworks-experiments/imagediff/pkg/image"
	imagediff_registry "github.com/weaveworks-experiments/imagediff/pkg/registry"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/docker/docker/pkg/term"
	"github.com/src-d/go-git/storage/memory"
	"golang.org/x/crypto/ssh/terminal"

	git "gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
)

// Options encapsulates the various options we can pass in to "diff" two container images.
type Options struct {
	DockerConfigPath string
}

// Diff diffs the provided images.
func Diff(x, y string, options Options) {
	docker, err := client.NewEnvClient()
	if err != nil {
		panic(err)
	}
	pull(docker, x, options.DockerConfigPath)
	pull(docker, y, options.DockerConfigPath)
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

func pull(docker *client.Client, imageName, dockerConfigPath string) {
	// Pulling images is pretty slow (i.e. takes a few seconds), even if the
	// image is already present locally. We therefore check if there are
	// already present locally first.
	exists, err := imageExistsLocally(docker, imageName)
	if err != nil {
		panic(err)
	}
	if exists {
		return
	}
	fmt.Printf("Pulling [%v]...\n", imageName)
	resp, err := docker.ImagePull(context.Background(), imageName, types.ImagePullOptions{
		PrivilegeFunc: func() (string, error) {
			fmt.Printf("Failed to pull %v: unauthorised.\n", imageName)
			return getDockerCredentials(dockerConfigPath, imageName)
		},
	})
	if err != nil {
		// Some registries (e.g. quay.io) return a 500 instead of a 403 for:
		//   "unauthorized: access to the requested resource is not authorized"
		// hence the above PrivilegeFunc will not be called, and we need to
		// provide credentials ourselves.
		if strings.Contains(err.Error(), "unauthorized:") {
			credentials, err := getDockerCredentials(dockerConfigPath, imageName)
			if err != nil {
				panic(err)
			}
			resp, err = docker.ImagePull(context.Background(), imageName, types.ImagePullOptions{
				RegistryAuth: credentials,
			})
			if err != nil {
				panic(err)
			}
		} else {
			panic(err)
		}
	}
	defer resp.Close()
	fd, isTerminal := term.GetFdInfo(ioutil.Discard)
	if err := jsonmessage.DisplayJSONMessagesStream(resp, ioutil.Discard, fd, isTerminal, nil); err != nil {
		panic(err)
	}
}

func imageExistsLocally(docker *client.Client, imageName string) (bool, error) {
	images, err := imageList(docker, imageName)
	if err != nil {
		return false, err
	}
	return len(images) > 0, nil
}

func imageList(docker *client.Client, imageName string) ([]types.ImageSummary, error) {
	args := filters.NewArgs()
	args.Add("reference", imageName)
	return docker.ImageList(context.Background(), types.ImageListOptions{
		Filters: args,
	})
}

func getDockerCredentials(dockerConfigPath, imageName string) (string, error) {
	if dockerConfigPath != "" {
		creds, err := getDockerCredentialsFrom(dockerConfigPath, imageName)
		if err == nil {
			return creds, nil
		}
	}
	creds, err := getDockerCredentialsFrom("~/.docker/config.json", imageName)
	if err == nil {
		return creds, nil
	}
	return askForCredentials(imageName)
}

func getDockerCredentialsFrom(dockerConfigPath, imageName string) (string, error) {
	fmt.Printf("Reading credentials from [%v]...\n", dockerConfigPath)
	configs, err := imagediff_registry.ReadAuthConfigs(dockerConfigPath)
	if err != nil {
		return "", err
	}
	img := image.Image(imageName)
	config, ok := configs[img.Registry()]
	if ok {
		encodedConfig, err := encodeAuthConfig(config)
		if err != nil {
			return "", err
		}
		return encodedConfig, nil
	}
	return "", errors.New("not found")
}

func askForCredentials(imageName string) (string, error) {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Enter your username: ")
	username, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	fmt.Print("Enter your password: ")
	passwordBytes, err := terminal.ReadPassword(0)
	if err != nil {
		return "", err
	}
	return encodeAuthConfig(types.AuthConfig{
		Username:      strings.TrimSpace(username),
		Password:      string(passwordBytes),
		ServerAddress: image.Image(imageName).Registry(),
	})
}

func encodeAuthConfig(authConfig types.AuthConfig) (string, error) {
	bytes, err := json.Marshal(authConfig)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(bytes), nil
}

func imageLabels(docker *client.Client, imageName string) map[string]string {
	inspect, _, err := docker.ImageInspectWithRaw(context.Background(), imageName)
	if err != nil {
		panic(err)
	}
	return inspect.Config.Labels
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
