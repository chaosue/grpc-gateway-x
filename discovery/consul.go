package discovery

import (
	kc "github.com/go-kratos/kratos/contrib/registry/consul/v2"
	"github.com/hashicorp/consul/api"
)

func NewConsul(cfg *api.Config) Discovery {
	client, err := api.NewClient(cfg)
	if err != nil {
		panic(err)
	}
	discovery := kc.New(client, kc.WithHealthCheck(true))
	return discovery
}
