package repository

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/src-d/go-git/storage/memory"
	"golang.org/x/crypto/ssh"
	git "gopkg.in/src-d/go-git.v4"
	git_ssh "gopkg.in/src-d/go-git.v4/plumbing/transport/ssh"
)

// GitRepository encapsulates data and behavior about a Git repository.
type GitRepository struct {
	Host         string
	Organization string
	Repository   string
}

// Options encapsulates the various options we can pass in to interact with a Git repository.
type Options struct {
	SSHPrivateKeyPath string
}

// HTTPS URL to clone this repository.
func (r GitRepository) HTTPS() string {
	return fmt.Sprintf("https://%v/%v/%v.git", r.Host, r.Organization, r.Repository)
}

// SSH endpoint to clone this repository.
func (r GitRepository) SSH() string {
	return fmt.Sprintf("git@%v:%v/%v.git", r.Host, r.Organization, r.Repository)
}

// Clone clones this repository in memory.
func (r GitRepository) Clone(options *Options) (*git.Repository, error) {
	logger := log.WithField("repository", r)
	logger.Info("cloning repository via HTTPS")
	storage := memory.NewStorage()
	repo, err := git.Clone(storage, nil, &git.CloneOptions{
		URL: r.HTTPS(),
	})
	if err != nil {
		if strings.Contains(err.Error(), "authentication required") {
			logger.WithField("err", err).Info("cloning via HTTPS failed, now retrying via SSH")
			sshKey, err := getOrDefaultPrivateSSHKey(options)
			if err != nil {
				return nil, err
			}
			repo, err = git.Clone(memory.NewStorage(), nil, &git.CloneOptions{
				URL:  r.SSH(),
				Auth: &git_ssh.PublicKeys{User: "git", Signer: sshKey},
			})
			if err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}
	return repo, nil
}

func getOrDefaultPrivateSSHKey(options *Options) (ssh.Signer, error) {
	sshKey, err := getOrDefaultPrivateSSHKeyBytes(options)
	if err != nil {
		return nil, err
	}
	signer, err := ssh.ParsePrivateKey(sshKey)
	if err != nil {
		return nil, err
	}
	return signer, nil
}

func getOrDefaultPrivateSSHKeyBytes(options *Options) ([]byte, error) {
	if options == nil || options.SSHPrivateKeyPath == "" {
		log.Debug("no private SSH key provided, using current user's default")
		return userDefaultPrivateSSHKey()
	}
	return privateSSHKeyFromPath(options.SSHPrivateKeyPath)
}

func userDefaultPrivateSSHKey() ([]byte, error) {
	homeDir, err := homeDir()
	if err != nil {
		return nil, err
	}
	path := fmt.Sprintf("%v/.ssh/id_rsa", homeDir)
	return privateSSHKeyFromPath(path)
}

func privateSSHKeyFromPath(path string) ([]byte, error) {
	path, err := expand(path)
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(path); err != nil {
		return nil, err
	}
	sshKey, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return sshKey, nil
}

func expand(path string) (string, error) {
	if strings.HasPrefix(path, "~") {
		homeDir, err := homeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(homeDir, path[1:]), nil
	}
	return path, nil
}

func homeDir() (string, error) {
	user, err := user.Current()
	if err != nil {
		return "", err
	}
	return user.HomeDir, nil
}

// New creates a new instance of GitRepository from the provided HTTPS URL or SSH connection string.
func New(url string) (*GitRepository, error) {
	host, org, repo, err := parseURL(url)
	if err != nil {
		return nil, err
	}
	return &GitRepository{
		Host:         host,
		Organization: org,
		Repository:   repo,
	}, nil
}

func parseURL(url string) (string, string, string, error) {
	httpsRegex, _ := regexp.Compile(`https://([^/]+)/([^/]+)/([^/]+).*?`)
	matches := httpsRegex.FindStringSubmatch(url)
	if len(matches) > 0 {
		return matches[1], matches[2], withoutDotGit(matches[3]), nil
	}
	sshRegex, _ := regexp.Compile(`git@([^:]+):([^/]+)/([^/]+).*?`)
	matches = sshRegex.FindStringSubmatch(url)
	if len(matches) > 0 {
		return matches[1], matches[2], withoutDotGit(matches[3]), nil
	}
	return "", "", "", fmt.Errorf("failed to parse URL: [%v]", url)
}

func withoutDotGit(repo string) string {
	if idx := strings.LastIndex(repo, ".git"); idx != -1 {
		return repo[:idx]
	}
	return repo
}
