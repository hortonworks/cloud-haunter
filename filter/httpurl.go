package operation

import (
	"os"

	ctx "github.com/hortonworks/cloud-haunter/context"
	"github.com/hortonworks/cloud-haunter/types"
	log "github.com/sirupsen/logrus"
)

type httpUrl struct {
	path string
	port string
}

func init() {
	// initialize new FilterType objects
	httpPathEnv := os.Getenv("HTTPURL_PATH")
	if len(httpPathEnv) < 1 {
		log.Warn("[HTTPURL] no path found in HTTPURL_PATH environment variable.")
	}
	httpPortEnv := os.Getenv("HTTPURL_PORT")
	if len(httpPortEnv) < 1 {
		log.Info("[HTTPURL] no port found in HTTPURL_PORT environment variable.")
	}
	log.Infof("[HTTPURL] path set to: %s, port set to: %s", httpPathEnv, httpPortEnv)
	ctx.Filters[types.HttpUrlFilter] = httpUrl{httpPathEnv, httpPortEnv}
}

func (f httpUrl) Execute(items []types.CloudItem) []types.CloudItem {
	log.Debugf("[HTTPURL] Filtering instances (%d): [%s]", len(items), items)
	return filter("HTTPURL", items, types.ExclusiveFilter, func(item types.CloudItem) bool {
		switch item.GetItem().(type) {
		case types.Instance:
			if item.GetItem().(types.Instance).State != types.Running {
				log.Debugf("[HTTPURL] Filter instance, because it's not in RUNNING state: %s", item.GetName())
				return false
			}
		default:
			log.Fatalf("[HTTPURL] Filter does not apply for cloud item: %s", item.GetName())
			return true
		}
		response := item.GetItem().(types.Instance).GetUrl(f.path, f.port)
		if response.Code >= 400 {
			return false
		}
		log.Debugf("[HTTPURL] %s: %s match, response body: %v", item.GetType(), item.GetName(), response.Body)
		return true
	})
}
