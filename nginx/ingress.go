package nginx

import (
	"encoding/json"
	"reflect"

	"k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/pkg/apis/extensions/v1beta1"
)

// IngressEx holds an Ingress along with Secrets and Endpoints of the services
// that are referenced in this Ingress
type IngressEx struct {
	Ingress   *v1beta1.Ingress
	Secrets   map[string]*v1.Secret
	Endpoints map[string][]string
}

func (i IngressEx) String() string {
	j, err := json.Marshal(i)
	if err != nil {
		return string("cant't marshal: " + reflect.TypeOf(i).String() + ", to json string, err: " + err.Error())
	}
	return string(j)
}
