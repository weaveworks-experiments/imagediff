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
	"os/user"
	"strings"

	"github.com/weaveworks-experiments/imagediff/pkg/image"
	imagediff_registry "github.com/weaveworks-experiments/imagediff/pkg/registry"
	"github.com/weaveworks-experiments/imagediff/pkg/repository"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/docker/docker/pkg/term"
	"github.com/src-d/go-git/storage/memory"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/terminal"

	git "gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
	git_ssh "gopkg.in/src-d/go-git.v4/plumbing/transport/ssh"

	log "github.com/sirupsen/logrus"
)

// Options encapsulates the various options we can pass in to "diff" two container images.
type Options struct {
	DockerConfigPath string
}

// Diff diffs the provided images.
func Diff(x, y string, options Options) ([]*Change, error) {
	docker, err := client.NewEnvClient()
	if err != nil {
		return nil, err
	}
	if err := pull(docker, x, options.DockerConfigPath); err != nil {
		return nil, err
	}
	if err := pull(docker, y, options.DockerConfigPath); err != nil {
		return nil, err
	}
	xLabels, err := imageLabels(docker, x)
	if err != nil {
		return nil, err
	}
	yLabels, err := imageLabels(docker, y)
	if err != nil {
		return nil, err
	}
	xRepo, xRev, err := repoAndRevision(xLabels)
	if err != nil {
		return nil, err
	}
	yRepo, yRev, err := repoAndRevision(yLabels)
	if err != nil {
		return nil, err
	}
	if err := validate(xRepo, yRepo); err != nil {
		return nil, err
	}
	r, err := gitClone(xRepo)
	if err != nil {
		return nil, err
	}
	xCommit, err := commit(r, xRev)
	if err != nil {
		return nil, err
	}
	yCommit, err := commit(r, yRev)
	if err != nil {
		return nil, err
	}
	return changeLog(xCommit, yCommit)
}

func pull(docker *client.Client, imageName, dockerConfigPath string) error {
	logger := log.WithFields(log.Fields{"image": imageName})
	// Pulling images is pretty slow (i.e. takes a few seconds), even if the
	// image is already present locally. We therefore check if there are
	// already present locally first.
	exists, err := imageExistsLocally(docker, imageName)
	if err != nil {
		return err
	}
	if exists {
		logger.Info("image already exists locally, nothing to pull")
		return nil
	}
	logger.Info("pulling image")
	resp, err := docker.ImagePull(context.Background(), imageName, types.ImagePullOptions{
		PrivilegeFunc: func() (string, error) {
			logger.Errorf("failed to pull image")
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
				return err
			}
			resp, err = docker.ImagePull(context.Background(), imageName, types.ImagePullOptions{
				RegistryAuth: credentials,
			})
			if err != nil {
				return err
			}
		} else {
			return err
		}
	}
	defer resp.Close()
	fd, isTerminal := term.GetFdInfo(ioutil.Discard)
	return jsonmessage.DisplayJSONMessagesStream(resp, ioutil.Discard, fd, isTerminal, nil)
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
	log.WithFields(log.Fields{"image": imageName, "path": dockerConfigPath}).Info("reading Docker credentials")
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

func imageLabels(docker *client.Client, imageName string) (map[string]string, error) {
	inspect, _, err := docker.ImageInspectWithRaw(context.Background(), imageName)
	if err != nil {
		return nil, err
	}
	return inspect.Config.Labels, nil
}

func repoAndRevision(labels map[string]string) (*repository.GitRepository, string, error) {
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
	repo, err := repository.New(vcsURL)
	if err != nil {
		return nil, "", err
	}
	if vcsRef == "" {
		return nil, "", errors.New("no revision")
	}
	return repo, vcsRef, nil
}

func validate(xRepo, yRepo *repository.GitRepository) error {
	if *xRepo != *yRepo {
		return fmt.Errorf("source code repositories do not match: %v != %v", xRepo, yRepo)
	}
	return nil
}

func gitClone(repo *repository.GitRepository) (*git.Repository, error) {
	logger := log.WithField("repository", *repo)
	logger.Info("cloning repository via HTTPS")
	storage := memory.NewStorage()
	r, err := git.Clone(storage, nil, &git.CloneOptions{
		URL: repo.HTTPS(),
	})
	if err != nil {
		if strings.Contains(err.Error(), "authentication required") {
			logger.WithField("err", err).Info("cloning via HTTPS failed, now retrying via SSH")
			usr, err := user.Current()
			if err != nil {
				return nil, err
			}
			sshKeyPath := fmt.Sprintf("%v/.ssh/id_rsa", usr.HomeDir)
			sshKey, err := ioutil.ReadFile(sshKeyPath)
			if err != nil {
				return nil, err
			}
			signer, err := ssh.ParsePrivateKey(sshKey)
			auth := &git_ssh.PublicKeys{User: "git", Signer: signer}
			r, err = git.Clone(memory.NewStorage(), nil, &git.CloneOptions{
				URL:  repo.SSH(),
				Auth: auth,
			})
			if err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}
	return r, nil
}

var errFound = errors.New("<Found>")

// Workaround to resolve a short hash to a full commit object.
// Once the following PR is merged, we should be able to do this in a more elegant way.
// See also: https://github.com/src-d/go-git/pull/706
func commit(r *git.Repository, shortHash string) (*object.Commit, error) {
	head, err := r.Head()
	if err != nil {
		return nil, err
	}
	commitsIter, err := r.Log(&git.LogOptions{From: head.Hash()})
	if err != nil {
		return nil, err
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
		return nil, fmt.Errorf("commit with short hash [%s] could not be found: %s", shortHash, err)
	}
	return commit, nil
}

// Change encapsulates the revision number and commit message for a code change.
type Change struct {
	Revision string
	Message  string
}

func changeLog(xCommit, yCommit *object.Commit) ([]*Change, error) {
	changeLog := []*Change{}
	err := object.NewCommitPostorderIter(yCommit, nil).ForEach(func(c *object.Commit) error {
		if c.Hash == xCommit.Hash {
			return errFound
		}
		changeLog = append(changeLog, &Change{Revision: c.Hash.String(), Message: c.Message})
		return nil
	})
	if err == nil || err != errFound {
		return nil, fmt.Errorf("commit with hash [%s] could not be found: %s", xCommit.Hash, err)
	}
	return changeLog, nil
}
