package aws

import (
	"os"

	ctx "github.com/hortonworks/cloud-haunter/context"
	"github.com/hortonworks/cloud-haunter/types"
	log "github.com/sirupsen/logrus"
)

var awsGovProvider = awsProvider{}

func init() {
	accessKeyId := os.Getenv("AWS_ACCESS_KEY_ID")
	if len(accessKeyId) == 0 {
		log.Warn("[AWS_GOV] AWS_ACCESS_KEY_ID environment variable is missing")
		return
	}
	secretAccessKey := os.Getenv("AWS_SECRET_ACCESS_KEY")
	if len(secretAccessKey) == 0 {
		log.Warn("[AWS_GOV] AWS_SECRET_ACCESS_KEY environment variable is missing")
		return
	}
	ctx.CloudProviders[types.AWS_GOV] = func() types.CloudProvider {
		if len(awsGovProvider.ec2Clients) == 0 {
			log.Debug("[AWS_GOV] Trying to prepare")
			ec2Client, err := newEc2Client("us-gov-west-1")
			if err != nil {
				panic("[AWS_GOV] Failed to create ec2 client, err: " + err.Error())
			}
			err = awsGovProvider.init(func() ([]string, error) {
				log.Debug("[AWS_GOV] Fetching regions")
				return getRegions(ec2Client)
			}, true)
			if err != nil {
				panic("[AWS_GOV] Failed to initialize provider, err: " + err.Error())
			}
			log.Info("[AWS_GOV] Successfully prepared")
		}
		return awsGovProvider
	}
}
