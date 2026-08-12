package operation

import (
	"strings"
	"testing"

	"github.com/hortonworks/cloud-haunter/types"
	"github.com/stretchr/testify/assert"
)

func TestParseCloudImagesJSON(t *testing.T) {
	t.Parallel()
	ci, err := parseCloudImagesJSON([]byte(`{"aws":{"ami-1":"us-east-1"},"azure":{"img-a":"westeu"},"gcp":{"g-1":"europe"}}`))

	assert.NoError(t, err)
	assert.Equal(t, map[string]string{"ami-1": "us-east-1"}, ci.Aws)
	assert.Equal(t, "westeu", ci.Azure["img-a"])
	assert.Equal(t, "europe", ci.Gcp["g-1"])
}

func TestParseCloudImagesJSONInvalid(t *testing.T) {
	t.Parallel()
	_, err := parseCloudImagesJSON([]byte("not json"))

	assert.Error(t, err)
}

func TestAppendToImages(t *testing.T) {
	t.Parallel()
	images := appendToImages(nil, map[string]string{"ami-1": "us-east-1"}, types.AWS)

	assert.Len(t, images, 1)
	assert.Equal(t, "ami-1", images[0].ID)
	assert.Equal(t, "us-east-1", images[0].Region)
	assert.Equal(t, types.AWS, images[0].CloudType)
}

func TestReadImagesExecute(t *testing.T) {
	t.Parallel()
	op := readImages{in: strings.NewReader(`{"aws":{"ami-1":"us-east-1"},"azure":{"img-a":"westeu"},"gcp":{"g-1":"europe"}}`)}

	items, err := op.Execute([]types.CloudType{types.AWS, types.AZURE, types.GCP})

	assert.NoError(t, err)
	assert.Len(t, items, 3)
	clouds := map[types.CloudType]bool{}
	for _, item := range items {
		clouds[item.GetCloudType()] = true
	}
	assert.True(t, clouds[types.AWS])
	assert.True(t, clouds[types.AZURE])
	assert.True(t, clouds[types.GCP])
}

// TestReadImagesExecuteSkipsUnsupportedCloud covers the default arm of the cloud
// switch: an unrecognized cloud is logged and contributes no images.
func TestReadImagesExecuteSkipsUnsupportedCloud(t *testing.T) {
	t.Parallel()
	op := readImages{in: strings.NewReader(`{"aws":{"ami-1":"us-east-1"}}`)}

	items, err := op.Execute([]types.CloudType{types.CloudType("BOGUS")})

	assert.NoError(t, err)
	assert.Empty(t, items)
}

// TestReadImagesExecuteOnlyRequestedClouds verifies images for clouds that were
// not requested are ignored, even when present in the input.
func TestReadImagesExecuteOnlyRequestedClouds(t *testing.T) {
	t.Parallel()
	op := readImages{in: strings.NewReader(`{"aws":{"ami-1":"us-east-1"},"gcp":{"g-1":"europe"}}`)}

	items, err := op.Execute([]types.CloudType{types.AWS})

	assert.NoError(t, err)
	assert.Len(t, items, 1)
	assert.Equal(t, types.AWS, items[0].GetCloudType())
}

func TestReadImagesExecuteErrorsOnEmptyInput(t *testing.T) {
	t.Parallel()
	op := readImages{in: strings.NewReader("")}

	_, err := op.Execute([]types.CloudType{types.AWS})

	assert.Error(t, err)
}

func TestReadImagesExecuteErrorsOnInvalidJSON(t *testing.T) {
	t.Parallel()
	op := readImages{in: strings.NewReader("not json")}

	_, err := op.Execute([]types.CloudType{types.AWS})

	assert.Error(t, err)
}
