package repository

import (
	"fmt"
	"regexp"
	"strings"
)

// GitRepository encapsulates data and behavior about a Git repository.
type GitRepository struct {
	Host         string
	Organization string
	Repository   string
}

// HTTPS URL to clone this repository.
func (r GitRepository) HTTPS() string {
	return fmt.Sprintf("https://%v/%v/%v.git", r.Host, r.Organization, r.Repository)
}

// SSH endpoint to clone this repository.
func (r GitRepository) SSH() string {
	return fmt.Sprintf("git@%v:%v/%v.git", r.Host, r.Organization, r.Repository)
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
