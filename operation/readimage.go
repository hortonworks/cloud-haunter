package operation

import (
	"bufio"
	"encoding/json"
	"errors"
	log "github.com/Sirupsen/logrus"
	ctx "github.com/hortonworks/cloud-haunter/context"
	"github.com/hortonworks/cloud-haunter/types"
	"io"
	"os"
)

func init() {
	ctx.Operations[types.ReadImages] = readImages{}
}

type readImages struct {
}

func (o readImages) Execute(clouds []types.CloudType) []types.CloudItem {
	log.Debugf("[READ_IMAGES] Collecting images from: [%s]", clouds)

	info, err := os.Stdin.Stat()
	if err != nil {
		panic(err)
	} else if info.Mode()&os.ModeCharDevice != 0 {
		panic(errors.New("[READ_IMAGES] standard input is not char device"))
	}

	reader := bufio.NewReader(os.Stdin)
	var output []rune
	for {
		input, _, err := reader.ReadRune()
		if err != nil && err == io.EOF {
			break
		}
		output = append(output, input)
	}
	if len(output) == 0 {
		panic("[READ_IMAGES] standard input is empty")
	}

	cloudImages, err := parseCloudImagesJSON([]byte(string(output)))
	if err != nil {
		panic(err)
	}

	images := []*types.Image{}
	for _, cloud := range clouds {
		switch cloud {
		case types.AWS:
			images = appendToImages(images, cloudImages.Aws, types.AWS)
			break
		case types.AZURE:
			images = appendToImages(images, cloudImages.Azure, types.AZURE)
			break
		case types.GCP:
			images = appendToImages(images, cloudImages.Gcp, types.GCP)
			break
		default:
			log.Warnf("[READ_IMAGES]  Cloud type not supported: %s", cloud.String())
		}
	}

	return convertToCloudItems(images)
}

type cloudImages struct {
	Aws   map[string]string `json:"aws,omitempty"`
	Azure map[string]string `json:"azure,omitempty"`
	Gcp   map[string]string `json:"gcp,omitempty"`
}

func parseCloudImagesJSON(raw []byte) (*cloudImages, error) {
	var cloudImages cloudImages
	err := json.Unmarshal(raw, &cloudImages)
	if err != nil {
		return nil, err
	}
	return &cloudImages, nil
}

func appendToImages(images []*types.Image, cloudImages map[string]string, cloudType types.CloudType) []*types.Image {
	for k, v := range cloudImages {
		images = append(images, &types.Image{ID: k, Region: v, CloudType: cloudType})
	}
	return images
}
