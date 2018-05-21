package repository_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/weaveworks-experiments/imagediff/pkg/repository"
)

func TestNewRepositoryFromSSH(t *testing.T) {
	r, err := repository.New("git@foo.com:bar/baz.git")
	assert.NoError(t, err)
	assert.NotNil(t, r)
	assert.Equal(t, "https://foo.com/bar/baz.git", r.HTTPS())
	assert.Equal(t, "git@foo.com:bar/baz.git", r.SSH())
}

func TestNewRepositoryFromHTTPS(t *testing.T) {
	r, err := repository.New("https://foo.com/bar/baz.git")
	assert.NoError(t, err)
	assert.NotNil(t, r)
	assert.Equal(t, "https://foo.com/bar/baz.git", r.HTTPS())
	assert.Equal(t, "git@foo.com:bar/baz.git", r.SSH())
}

func TestNewRepositoryFromHTTPSWithoutDotGit(t *testing.T) {
	r, err := repository.New("https://foo.com/bar/baz")
	assert.NoError(t, err)
	assert.NotNil(t, r)
	assert.Equal(t, "https://foo.com/bar/baz.git", r.HTTPS())
	assert.Equal(t, "git@foo.com:bar/baz.git", r.SSH())
}

func TestNewRepositoryFromHTTPSWithoutDotGitWithPath(t *testing.T) {
	r, err := repository.New("https://foo.com/bar/baz/tree/master/path/to/some/dir")
	assert.NoError(t, err)
	assert.NotNil(t, r)
	assert.Equal(t, "https://foo.com/bar/baz.git", r.HTTPS())
	assert.Equal(t, "git@foo.com:bar/baz.git", r.SSH())
}

func TestNewRepositoryFromHTTPSWithPath(t *testing.T) {
	r, err := repository.New("https://foo.com/bar/baz.git/tree/master/path/to/some/dir")
	assert.NoError(t, err)
	assert.NotNil(t, r)
	assert.Equal(t, "https://foo.com/bar/baz.git", r.HTTPS())
	assert.Equal(t, "git@foo.com:bar/baz.git", r.SSH())
}

func TestNewRepositoryFromInvalidString(t *testing.T) {
	r, err := repository.New("g0t r00t?")
	assert.Error(t, err, "failed to parse [g0t r00t?]")
	assert.Nil(t, r)
}
