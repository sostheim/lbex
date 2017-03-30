/*
Copyright 2015 The Kubernetes Authors.

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

package annotations

import (
	"strconv"

	"github.com/golang/glog"
	"k8s.io/client-go/pkg/api"
	"k8s.io/client-go/pkg/api/v1"
)

const (
	// LBAnnotationKey picks a specific "class" for the specified load balancer.
	LBAnnotationKey = "kubernetes.io/loadbalancer.class"

	// LBEXValue - this controller only processes Services with this annotation.
	LBEXValue = "loadbalancer.lbex"

	// LBEXAlgorithmKey - requested load balancing algorithm
	LBEXAlgorithmKey = "loadbalancer.lbex/algorithm"

	// LBEXMethodKey - Algorithm Least Time has an arugment "Method"
	LBEXMethodKey = "loadbalancer.lbex/method"

	// LBEXHostKey - the load balancer hostname
	LBEXHostKey = "loadbalancer.lbex/host"

	// LBEXResolverKey - DNS Resolver for DNS based service names (if any)
	LBEXResolverKey = "loadbalancer.lbex/resolver"
)

// serviceAnnotations - map of key:value annotations discoverd for LBEX
type serviceAnnotations map[string]string

func (as serviceAnnotations) parseBool(name string) (bool, error) {
	val, ok := as[name]
	if ok {
		b, err := strconv.ParseBool(val)
		if err != nil {
			return false, NewInvalidAnnotationContent(name, val)
		}
		return b, nil
	}
	return false, ErrMissingAnnotations
}

func (as serviceAnnotations) parseString(name string) (string, error) {
	val, ok := as[name]
	if ok {
		return val, nil
	}
	return "", ErrMissingAnnotations
}

func (as serviceAnnotations) parseInt(name string) (int, error) {
	val, ok := as[name]
	if ok {
		i, err := strconv.Atoi(val)
		if err != nil {
			return 0, NewInvalidAnnotationContent(name, val)
		}
		return i, nil
	}
	return 0, ErrMissingAnnotations
}

func checkAnnotationAPI(name string, service *api.Service) error {
	if service == nil || len(service.GetAnnotations()) == 0 {
		return ErrMissingAnnotations
	}
	if name == "" {
		return ErrInvalidAnnotationName
	}
	return nil
}

func checkAnnotationV1(name string, service *v1.Service) error {
	if service == nil || len(service.GetAnnotations()) == 0 {
		return ErrMissingAnnotations
	}
	if name == "" {
		return ErrInvalidAnnotationName
	}
	return nil
}

func checkAnnotation(name string, obj interface{}) error {
	if name == "" {
		return ErrInvalidAnnotationName
	}
	switch t := obj.(type) {
	default:
		glog.V(3).Infof("unexpected type assertion value: %T\n", t)
	case *v1.Service:
		service := obj.(*v1.Service)
		if len(service.GetAnnotations()) == 0 {
			return ErrMissingAnnotations
		}
	case *api.Service:
		service := obj.(*api.Service)
		if len(service.GetAnnotations()) == 0 {
			return ErrMissingAnnotations
		}
	}
	return nil
}

// GetBoolAnnotationAPI extracts a boolean from an api.Service annotation
func GetBoolAnnotationAPI(name string, service *api.Service) (bool, error) {
	err := checkAnnotation(name, service)
	if err != nil {
		return false, err
	}
	return serviceAnnotations(service.GetAnnotations()).parseBool(name)
}

// GetBoolAnnotation extracts a boolean from service annotation
func GetBoolAnnotation(name string, obj interface{}) (bool, error) {
	err := checkAnnotation(name, obj)
	if err != nil {
		return false, err
	}
	switch obj.(type) {
	case *v1.Service:
		service := obj.(*v1.Service)
		return serviceAnnotations(service.GetAnnotations()).parseBool(name)
	case *api.Service:
		service := obj.(*api.Service)
		return serviceAnnotations(service.GetAnnotations()).parseBool(name)
	}
	return false, ErrMissingAnnotations
}

// GetStringAnnotation extracts a string from service annotation
func GetStringAnnotation(name string, obj interface{}) (string, error) {
	err := checkAnnotation(name, obj)
	if err != nil {
		return "", err
	}
	switch obj.(type) {
	case *v1.Service:
		service := obj.(*v1.Service)
		return serviceAnnotations(service.GetAnnotations()).parseString(name)
	case *api.Service:
		service := obj.(*api.Service)
		return serviceAnnotations(service.GetAnnotations()).parseString(name)
	}
	return "", ErrMissingAnnotations
}

// GetIntAnnotation extracts an int from an Ingress annotation
func GetIntAnnotation(name string, obj interface{}) (int, error) {
	err := checkAnnotation(name, obj)
	if err != nil {
		return 0, err
	}
	switch obj.(type) {
	case *v1.Service:
		service := obj.(*v1.Service)
		return serviceAnnotations(service.GetAnnotations()).parseInt(name)
	case *api.Service:
		service := obj.(*api.Service)
		return serviceAnnotations(service.GetAnnotations()).parseInt(name)
	}
	return 0, ErrMissingAnnotations
}

// GetAlgorithm returns the string value of the annotations, or
// the empty string if not present, and a bool to indicate wether
// or not the value was present
func GetAlgorithm(obj interface{}) (string, bool) {
	value, err := GetStringAnnotation(LBEXAlgorithmKey, obj)
	if err != nil && !IsMissingAnnotations(err) {
		glog.V(3).Infof("unexpected error reading annotation: %v", err)
		return "", false
	}
	return value, true
}

// GetMethod returns the string value of the annotations, or
// the empty string if not present, and a bool to indicate wether
// or not the value was present
func GetMethod(obj interface{}) (string, bool) {
	value, err := GetStringAnnotation(LBEXMethodKey, obj)
	if err != nil && !IsMissingAnnotations(err) {
		glog.V(3).Infof("unexpected error reading annotation: %v", err)
		return "", false
	}
	return value, true
}

// GetHost returns the string value of the annotations, or the
// empty string if not present, and a bool to indicate wether
// or not the value was present
func GetHost(obj interface{}) (string, bool) {
	value, err := GetStringAnnotation(LBEXHostKey, obj)
	if err != nil && !IsMissingAnnotations(err) {
		glog.V(3).Infof("unexpected error reading annotation: %v", err)
		return "", false
	}
	return value, true
}

// GetResolver returns the string value of the annotations, or the
// empty string if not present, and a bool to indicate wether
// or not the value was present
func GetResolver(obj interface{}) (string, bool) {
	value, err := GetStringAnnotation(LBEXResolverKey, obj)
	if err != nil && !IsMissingAnnotations(err) {
		glog.V(3).Infof("unexpected error reading annotation: %v", err)
		return "", false
	}
	return value, true
}

// IsValid returns true if the given Service object specifies 'lbex' as the value
// to the loadbalancer.class annotation.
func IsValid(obj interface{}) bool {
	value, err := GetStringAnnotation(LBAnnotationKey, obj)
	if err != nil && !IsMissingAnnotations(err) {
		glog.V(3).Infof("unexpected error reading annotation: %v", err)
	}
	return value == LBEXValue
}
