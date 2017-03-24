/*
Copyright 2016 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"errors"
	"fmt"

	"github.com/golang/glog"

	"k8s.io/client-go/pkg/api"
	"k8s.io/client-go/pkg/api/v1"
)

var (
	lbAPIPort = 8081
)

// Service models a backend service entry in the load balancer config.
// The Ep field can contain the ips of the pods that make up a service, or the
// clusterIP of the service itself (in which case the list has a single entry,
// and kubernetes handles loadbalancing across the service endpoints).
type Service struct {
	Name string
	Ep   []string

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

// ValidateServiceObjectType return wether or not the given object
// is of type *api.Service or *v1.Service -> valid true, valid false otherwise
func ValidateServiceObjectType(obj interface{}) error {
	switch obj.(type) {
	case *v1.Service:
		return nil
	case *api.Service:
		return nil
	}
	return errors.New("unexpected type")
}

// GetServiceName return validated service type's name, error otherwise.
func GetServiceName(obj interface{}) (string, error) {
	switch t := obj.(type) {
	default:
		glog.V(3).Infof("GetServiceName: unexpected type assertion value: %T\n", t)
	case *v1.Service:
		service := obj.(*v1.Service)
		return string(service.Name), nil
	case *api.Service:
		service := obj.(*api.Service)
		return string(service.Name), nil
	}
	return "", errors.New("unexpected type")
}

// GetServiceNamespace return validated service type's namespace, error otherwise.
func GetServiceNamespace(obj interface{}) (string, error) {
	switch t := obj.(type) {
	default:
		glog.V(3).Infof("GetServiceNamespace: unexpected type assertion value: %T\n", t)
	case *v1.Service:
		service := obj.(*v1.Service)
		return string(service.Namespace), nil
	case *api.Service:
		service := obj.(*api.Service)
		return string(service.Namespace), nil
	}
	return "", errors.New("unexpected type")
}

// GetServiceType return validated service type's Tupe, error otherwise.
func GetServiceType(obj interface{}) (string, error) {
	switch t := obj.(type) {
	default:
		glog.V(3).Infof("GetServiceType: unexpected type assertion value: %T\n", t)
	case *v1.Service:
		service := obj.(*v1.Service)
		return string(service.Spec.Type), nil
	case *api.Service:
		service := obj.(*api.Service)
		return string(service.Spec.Type), nil
	}
	return "", errors.New("unexpected type")
}

// ServiceTypeLoadBalancer returns true iff "Type: LoadBalancer"
func ServiceTypeLoadBalancer(obj interface{}) bool {
	serviceType, err := GetServiceType(obj)
	if err != nil {
		return false
	}
	switch obj.(type) {
	case *v1.Service:
		return serviceType == string(v1.ServiceTypeLoadBalancer)
	case *api.Service:
		return serviceType == string(api.ServiceTypeLoadBalancer)
	}
	return false
}

// GetServicePortTargetPortInt returns the numeric value of TargetPort
func GetServicePortTargetPortInt(obj interface{}) (int, error) {
	switch t := obj.(type) {
	default:
		glog.V(3).Infof("GetServicePortTargetPortInt: unexpected type assertion value: %T\n", t)
	case *v1.ServicePort:
		servicePort := obj.(*v1.ServicePort)
		return servicePort.TargetPort.IntValue(), nil
	case *api.Service:
		servicePort := obj.(*api.ServicePort)
		return servicePort.TargetPort.IntValue(), nil
	}
	return 0, errors.New("unexpected type")
}

// GetServicePortTargetPortString returns the numeric value of TargetPort
func GetServicePortTargetPortString(obj interface{}) (string, error) {
	switch t := obj.(type) {
	default:
		glog.V(3).Infof("GetServicePortTargetPortString: unexpected type assertion value: %T\n", t)
	case *v1.ServicePort:
		servicePort := obj.(*v1.ServicePort)
		return servicePort.TargetPort.StrVal, nil
	case *api.Service:
		servicePort := obj.(*api.ServicePort)
		return servicePort.TargetPort.StrVal, nil
	}
	return "", errors.New("unexpected type")
}

// GetServiceNameForLBRule - convenience type name modifications for lb rules.
func GetServiceNameForLBRule(serviceName string, servicePort int) string {
	if servicePort == 80 {
		return serviceName
	}
	return fmt.Sprintf("%v:%v", serviceName, servicePort)
}
