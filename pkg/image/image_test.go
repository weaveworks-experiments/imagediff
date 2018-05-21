package image_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/weaveworks-experiments/imagediff/pkg/image"
)

func TestRegistry(t *testing.T) {
	assert.Equal(t, "docker.io", image.Image("owner/image:tag").Registry())
	assert.Equal(t, "quay.io", image.Image("quay.io/owner/image:tag").Registry())
}
