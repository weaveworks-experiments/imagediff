package registry

import (
	"encoding/json"
	"errors"
	"io/ioutil"

	"github.com/docker/docker/api/types"
)

// AuthConfigs groups Docker authentication configuration objects, keyed by registry.
type AuthConfigs map[string]types.AuthConfig

// ReadAuthConfigs reads and deserializes the provided Docker config.json file
func ReadAuthConfigs(dockerConfigPath string) (AuthConfigs, error) {
	bytes, err := ioutil.ReadFile(dockerConfigPath)
	if err != nil {
		return nil, err
	}
	var configs struct {
		Auths AuthConfigs `json:"auths"`
	}
	err = json.Unmarshal(bytes, &configs)
	if err != nil {
		return nil, err
	}
	return configs.Auths, nil
}

// ReadAuthConfig reads and deserializes the provided Docker config.json file, and extracts the configuration for the provided registry.
func ReadAuthConfig(dockerConfigPath, registry string) (types.AuthConfig, error) {
	configs, err := ReadAuthConfigs(dockerConfigPath)
	if err != nil {
		return types.AuthConfig{}, err
	}
	config, ok := configs[registry]
	if ok {
		return config, nil
	}
	return types.AuthConfig{}, errors.New("not found")
}
