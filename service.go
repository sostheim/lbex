package main

import (
	"errors"
	"fmt"
	"strings"

	"github.com/golang/glog"
	"github.com/sostheim/lbex/annotations"

	"k8s.io/client-go/pkg/api"
	v1 "k8s.io/client-go/pkg/api/v1"
)

const noneString = "none"

// Endpoint models all the information needed to target an endpoint.
type Endpoint struct {
	// ServicePort - the port that to listen on for the service's external clients
	ServicePort int
	// NodeIP - the IP address of a host/worker node
	NodeIP string
	// NodeName - the name of a host/worker node
	NodeName string
	// NodePort - the port that the host/worker node listens on for fowarding to the pod/ip:port
	NodePort int
	// PortName - the name of the port if present, or 'unnamed' otherwise
	PortName string
	// PodIP - the pods ip address
	PodIP string
	// PodPort - the port the that the pod listens on
	PodPort int
	// Protocol - TCP or UDP
	Protocol string
}

// Service models a backend service entry in the load balancer config.
// The Ep field can contain the ips of the pods that make up a service, or the
// clusterIP of the service itself (in which case the list has a single entry,
// and kubernetes handles loadbalancing across the service endpoints).
type Service struct {
	Name string

	// TODO: remove deprecated field
	// Deprecated: left intact for backwards compatibility for Ingress
	Ep []string

	Endpoints []Endpoint

	// Kubernetes endpoint port.
	BackendPort int

	// FrontendPort is the port that the loadbalancer listens on for traffic
	// for this service. For each tcp service it is the service port of any
	// service matching a name in the tcpServices set.
	FrontendPort int

	// Host if not empty it will add a new
	Host string

	// Algorithm
	Algorithm string
}

type serviceByName []Service

func (s serviceByName) Len() int {
	return len(s)
}
func (s serviceByName) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}
func (s serviceByName) Less(i, j int) bool {
	return s[i].Name < s[j].Name
}

// ValidateServiceObject returns true iff:
// - the object is of a valid v1 API Service object
// - is a service type we provide load balancing for
// - has a valid annotation indicating the lbex class
// returns false otherwise
func ValidateServiceObject(obj interface{}) bool {
	err := ValidateServiceObjectType(obj)
	if err != nil {
		glog.V(4).Infof("can't validate service object type, err: " + err.Error())
		return false
	}
	if !IsValidServiceType(obj) {
		glog.V(4).Infof("can't validate based on service type")
		return false
	}
	if !annotations.IsValid(obj) {
		glog.V(4).Infof("can't validate based on annotations")
		return false
	}
	return true
}

// ValidateServiceObjectType return wether or not the given object
// is of type *api.Service or *v1.Service -> valid true, valid false otherwise
func ValidateServiceObjectType(obj interface{}) error {
	switch obj.(type) {
	case *v1.Service:
		return nil
	case *api.Service:
		return errors.New("ValidateServiceObjectType: unsupported type api.* (must be v1.*)")
	}
	return errors.New("ValidateServiceObjectType: unexpected type")
}

// GetServiceName return validated service type's name, error otherwise.
func GetServiceName(obj interface{}) (string, error) {
	service, ok := obj.(*v1.Service)
	if !ok {
		return "", errors.New("type assertion failure")
	}
	return string(service.Name), nil

}

// GetServiceNamespace return validated service type's namespace, error otherwise.
func GetServiceNamespace(obj interface{}) (string, error) {
	service, ok := obj.(*v1.Service)
	if !ok {
		return "", errors.New("type assertion failure")
	}
	return string(service.Namespace), nil
}

// GetServiceType return validated service type's Type string, error otherwise.
func GetServiceType(obj interface{}) (string, error) {
	service, ok := obj.(*v1.Service)
	if !ok {
		return "", errors.New("type assertion failure")
	}
	return string(service.Spec.Type), nil
}

// ServiceTypeLoadBalancer returns true iff "Type: LoadBalancer"
func ServiceTypeLoadBalancer(obj interface{}) bool {
	serviceType, err := GetServiceType(obj)
	if err != nil {
		return false
	}
	return serviceType == string(api.ServiceTypeLoadBalancer)
}

// ServiceTypeNodePort returns true iff "Type: NodePort"
func ServiceTypeNodePort(obj interface{}) bool {
	serviceType, err := GetServiceType(obj)
	if err != nil {
		return false
	}
	return serviceType == string(api.ServiceTypeNodePort)
}

// ServiceTypeClusterIP returns true iff "Type: ClusterIP"
func ServiceTypeClusterIP(obj interface{}) bool {
	serviceType, err := GetServiceType(obj)
	if err != nil {
		return false
	}
	return serviceType == string(api.ServiceTypeClusterIP)
}

// ServiceTypeExternalName returns true iff "Type: ExternalName"
func ServiceTypeExternalName(obj interface{}) bool {
	serviceType, err := GetServiceType(obj)
	if err != nil {
		return false
	}
	return serviceType == string(api.ServiceTypeExternalName)
}

// GetClusterIP returns the services cluster ip value as a string, or an error
func GetClusterIP(obj interface{}) (string, error) {
	service, ok := obj.(*v1.Service)
	if !ok {
		return "", errors.New("type assertion failure")
	}
	return string(service.Spec.ClusterIP), nil
}

// ServiceTypeHeadless returns true iff ClusterIP is set to "None"
func ServiceTypeHeadless(obj interface{}) bool {
	clusterIP, err := GetClusterIP(obj)
	if err != nil {
		return false
	}
	return strings.EqualFold(clusterIP, noneString)
}

// IsValidServiceType returns true iff service properties are suche that
// external load balancing is appropriate
func IsValidServiceType(obj interface{}) bool {
	if !ServiceTypeHeadless(obj) &&
		(ServiceTypeNodePort(obj) || ServiceTypeClusterIP(obj) || ServiceTypeLoadBalancer(obj)) {
		return true
	}
	return false
}

// GetServicePortTargetPortInt returns the numeric value of TargetPort
func GetServicePortTargetPortInt(obj interface{}) (int, error) {
	servicePort, ok := obj.(*v1.ServicePort)
	if !ok {
		return 0, errors.New("type assertion failure")
	}
	return servicePort.TargetPort.IntValue(), nil
}

// GetServicePortTargetPortString returns the numeric value of TargetPort
func GetServicePortTargetPortString(obj interface{}) (string, error) {
	servicePort, ok := obj.(*v1.ServicePort)
	if !ok {
		return "", errors.New("type assertion failure")
	}
	return servicePort.TargetPort.StrVal, nil
}

// GetServiceNameForLBRule - convenience type name modifications for lb rules.
func GetServiceNameForLBRule(serviceName string, servicePort int) string {
	if servicePort == 80 {
		return serviceName
	}
	return fmt.Sprintf("%v:%v", serviceName, servicePort)
}
