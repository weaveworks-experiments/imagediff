package image

import (
	"github.com/docker/distribution/reference"
	"github.com/docker/docker/registry"
)

// Image is the abstraction for a Docker image.
type Image string

// Registry extracts the registry from this Docker image.
func (image Image) Registry() string {
	distributionRef, err := reference.ParseNormalizedNamed(string(image))
	if err != nil {
		return ""
	}
	repoInfo, err := registry.ParseRepositoryInfo(distributionRef)
	if err != nil {
		return ""
	}
	return repoInfo.Index.Name
}
