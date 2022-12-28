package discovery

import "github.com/go-kratos/kratos/v2/registry"

type Discovery interface {
	registry.Discovery
	ListServices() (allServices map[string][]*registry.ServiceInstance, err error)
}
