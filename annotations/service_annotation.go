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
)

const (
	// LBAnnotationKey picks a specific "class" for the specified load balancer.
	LBAnnotationKey = "kubernetes.io/loadbalancer.class"

	// LBEXValue - this controller only processes Services with this annotation.
	LBEXValue = "loadbalancer.lbex"

	// LBEXAlgorithmKey - requested load balancing algorithm
	LBEXAlgorithmKey = "loadbalancer.lbex/algorithm"

	// LBEXHostKey - the load balancer hostname
	LBEXHostKey = "loadbalancer.lbex/host"
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

func checkAnnotation(name string, service *api.Service) error {
	if service == nil || len(service.GetAnnotations()) == 0 {
		return ErrMissingAnnotations
	}
	if name == "" {
		return ErrInvalidAnnotationName
	}

	return nil
}

// GetBoolAnnotation extracts a boolean from service annotation
func GetBoolAnnotation(name string, service *api.Service) (bool, error) {
	err := checkAnnotation(name, service)
	if err != nil {
		return false, err
	}
	return serviceAnnotations(service.GetAnnotations()).parseBool(name)
}

// GetStringAnnotation extracts a string from service annotation
func GetStringAnnotation(name string, service *api.Service) (string, error) {
	err := checkAnnotation(name, service)
	if err != nil {
		return "", err
	}
	return serviceAnnotations(service.GetAnnotations()).parseString(name)
}

// GetIntAnnotation extracts an int from an Ingress annotation
func GetIntAnnotation(name string, service *api.Service) (int, error) {
	err := checkAnnotation(name, service)
	if err != nil {
		return 0, err
	}
	return serviceAnnotations(service.GetAnnotations()).parseInt(name)
}

// GetAlgorithm returns the string value of the annotations, or
// the empty string if not present, and a bool to indicate wether
// or not the value was present
func GetAlgorithm(service *api.Service) (string, bool) {
	value, err := GetStringAnnotation(LBEXAlgorithmKey, service)
	if err != nil {
		glog.Warningf("unexpected error reading annotation: %v", err)
		return "", false
	}
	return value, true
}

// GetHost returns the string value of the annotations, or the
// empty string if not present, and a bool to indicate wether
// or not the value was present
func GetHost(service *api.Service) (string, bool) {
	value, err := GetStringAnnotation(LBEXHostKey, service)
	if err != nil {
		glog.Warningf("unexpected error reading annotation: %v", err)
		return "", false
	}
	return value, true
}

// IsValid returns true if the given Service object specifies 'lbex' as the value
// to the loadbalancer.class annotation.
func IsValid(service *api.Service) bool {
	value, err := GetStringAnnotation(LBAnnotationKey, service)
	if err != nil && !IsMissingAnnotations(err) {
		glog.Warningf("unexpected error reading annotation: %v", err)
	}
	return value == LBEXValue
}
