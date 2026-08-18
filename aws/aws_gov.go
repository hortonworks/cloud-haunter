package aws

import (
	"sync"

	"github.com/hortonworks/cloud-haunter/config"
	"github.com/hortonworks/cloud-haunter/types"
	log "github.com/sirupsen/logrus"
)

// registerGov wires the AWS GOV provider into the registry with cfg injected.
// Called from Register.
func registerGov(cfg *config.Config) {
	if !awsCredentialsPresent("[AWS_GOV]") {
		return
	}
	p := &awsProvider{cfg: cfg}
	var once sync.Once
	cfg.CloudProviders[types.AWS_GOV] = func() types.CloudProvider {
		once.Do(func() {
			log.Debug("[AWS_GOV] Trying to prepare")
			ec2Client, err := newEc2Client("us-gov-west-1")
			if err != nil {
				panic("[AWS_GOV] Failed to create ec2 client, err: " + err.Error())
			}
			err = p.init(func() ([]string, error) {
				log.Debug("[AWS_GOV] Fetching regions")
				return getRegions(cfg, ec2Client)
			}, true)
			if err != nil {
				panic("[AWS_GOV] Failed to initialize provider, err: " + err.Error())
			}
			log.Info("[AWS_GOV] Successfully prepared")
		})
		return p
	}
}
