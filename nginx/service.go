package nginx

import "k8s.io/client-go/pkg/api/v1"

// ServiceSpec holds an Service and the Endpoints of the services
type ServiceSpec struct {
	Service   *v1.Service
	Endpoints map[string][]string
}
