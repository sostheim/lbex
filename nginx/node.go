package nginx

import (
	"encoding/json"
	"reflect"
)

// Node models a k8s worker node's id and addresses
type Node struct {
	Name       string
	Hostname   string
	ExternalIP string
	InternalIP string
	Active     bool
}

func (n Node) String() string {
	j, err := json.Marshal(n)
	if err != nil {
		return string("cant't marshal: " + reflect.TypeOf(n).String() + ", to json string, err: " + err.Error())
	}
	return string(j)
}
