package nginx

import (
	"encoding/json"
	"reflect"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apis/extensions"
)

// IngressEx holds an Ingress along with Secrets and Endpoints of the services
// that are referenced in this Ingress
type IngressEx struct {
	Ingress   *extensions.Ingress
	Secrets   map[string]*api.Secret
	Endpoints map[string][]string
}

func (i IngressEx) String() string {
	j, err := json.Marshal(i)
	if err != nil {
		return string("cant't marshal: " + reflect.TypeOf(i).String() + ", to json string, err: " + err.Error())
	}
	return string(j)
}
