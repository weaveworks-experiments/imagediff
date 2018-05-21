package registry_test

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/stretchr/testify/assert"
	"github.com/weaveworks-experiments/imagediff/pkg/registry"
)

func TestReadAuthConfigs(t *testing.T) {
	// Setup:
	path := sampleDockerConfig(t)
	defer os.Remove(path)

	configs, err := registry.ReadAuthConfigs(path)
	assert.NoError(t, err)
	assert.Equal(t, registry.AuthConfigs(map[string]types.AuthConfig{
		"https://index.docker.io/v1/": types.AuthConfig{
			Auth: "Zm9vOmJhego=",
		},
		"quay.io": types.AuthConfig{
			Auth: "Zm9vOmJhcgo=",
		},
	}), configs)
}

func TestReadAuthConfig(t *testing.T) {
	// Setup:
	path := sampleDockerConfig(t)
	defer os.Remove(path)

	config, err := registry.ReadAuthConfig(path, "quay.io")
	assert.NoError(t, err)
	assert.Equal(t, types.AuthConfig{
		Auth: "Zm9vOmJhcgo=",
	}, config)

	config, err = registry.ReadAuthConfig(path, "non-existing-registry")
	assert.Error(t, err, "not found")
}

func sampleDockerConfig(t *testing.T) string {
	path, err := tempFile(t, `{
		"auths": {
			"https://index.docker.io/v1/": {
				"auth": "Zm9vOmJhego="
			},
			"quay.io": {
				"auth": "Zm9vOmJhcgo="
			}
		},
		"HttpHeaders": {
			"User-Agent": "Docker-Client/18.03.1-ce (linux)"
		}
	}`)
	assert.NoError(t, err)
	return path
}

func tempFile(t *testing.T, data string) (string, error) {
	f, err := ioutil.TempFile("", "")
	if err != nil {
		return "", err
	}
	if _, err := f.WriteString(data); err != nil {
		return "", err
	}
	if err := f.Close(); err != nil {
		return "", err
	}
	return f.Name(), nil
}
