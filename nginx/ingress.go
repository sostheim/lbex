package nginx

import "k8s.io/kubernetes/pkg/api"
import "k8s.io/kubernetes/pkg/apis/extensions"

// IngressEx holds an Ingress along with Secrets and Endpoints of the services
// that are referenced in this Ingress
type IngressEx struct {
	Ingress   *extensions.Ingress
	Secrets   map[string]*api.Secret
	Endpoints map[string][]string
}
