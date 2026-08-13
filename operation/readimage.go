package operation

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"os"

	"github.com/hortonworks/cloud-haunter/types"
	log "github.com/sirupsen/logrus"
)

// NewReadImages returns the readImages operation implementation reading from
// stdin.
func NewReadImages() types.Operation {
	return readImages{in: os.Stdin}
}

type readImages struct {
	// in is the source the image JSON is read from; injectable so tests supply an
	// in-memory reader instead of swapping the os.Stdin global (which would force
	// the tests to run non-parallel).
	in io.Reader
}

func (o readImages) Execute(clouds []types.CloudType) ([]types.CloudItem, error) {
	log.Debugf("[READ_IMAGES] Collecting images from: [%s]", clouds)

	// Guard against an interactive terminal (reading would block forever). Only
	// *os.File exposes Stat(); an injected reader skips the check.
	if f, ok := o.in.(interface{ Stat() (os.FileInfo, error) }); ok {
		info, err := f.Stat()
		if err != nil {
			return nil, err
		} else if info.Mode()&os.ModeCharDevice != 0 {
			return nil, errors.New("[READ_IMAGES] standard input is not char device")
		}
	}

	reader := bufio.NewReader(o.in)
	var output []rune
	for {
		input, _, err := reader.ReadRune()
		if err != nil && err == io.EOF {
			break
		}
		output = append(output, input)
	}
	if len(output) == 0 {
		return nil, errors.New("[READ_IMAGES] standard input is empty")
	}

	cloudImages, err := parseCloudImagesJSON([]byte(string(output)))
	if err != nil {
		return nil, err
	}

	images := []*types.Image{}
	for _, cloud := range clouds {
		switch cloud {
		case types.AWS:
			images = appendToImages(images, cloudImages.Aws, types.AWS)
		case types.AZURE:
			images = appendToImages(images, cloudImages.Azure, types.AZURE)
		case types.GCP:
			images = appendToImages(images, cloudImages.Gcp, types.GCP)
		default:
			log.Warnf("[READ_IMAGES]  Cloud type not supported: %s", cloud.String())
		}
	}

	return convertToCloudItems(images), nil
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
