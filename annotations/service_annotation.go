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
	"errors"
	"reflect"
	"strconv"

	"github.com/golang/glog"
	"k8s.io/client-go/pkg/api/v1"
)

const (
	// LBEXClassKey picks a specific "class" for the specified load balancer.
	LBEXClassKey = "kubernetes.io/loadbalancer-class"

	// LBEXClassKeyValue - this controller only processes Services with this annotation.
	LBEXClassKeyValue = "loadbalancer-lbex"

	// LBEXAlgorithmKey - requested load balancing algorithm
	LBEXAlgorithmKey = "loadbalancer.lbex/algorithm"

	// LBEXMethodKey - Algorithm Least Time has an arugment "Method"
	LBEXMethodKey = "loadbalancer.lbex/method"

	// LBEXHostKey - the load balancer hostname
	LBEXHostKey = "loadbalancer.lbex/host"

	// LBEXResolverKey - DNS Resolver for DNS based service names (if any)
	LBEXResolverKey = "loadbalancer.lbex/resolver"

	// LBEXUpstreamType - upstream server target type
	LBEXUpstreamType = "loadbalancer.lbex/upstream-type"

	// LBEXNodeAddressType - node address type
	LBEXNodeAddressType = "loadbalancer.lbex/node-address-type"

	// LBEXNodeSet - set of nodes to load balance across
	LBEXNodeSet = "loadbalancer.lbex/node-set"
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

func checkAnnotation(name string, obj interface{}) error {
	if name == "" {
		return ErrInvalidAnnotationName
	}
	switch t := obj.(type) {
	default:
		return errors.New("unexpected type value: " + reflect.TypeOf(t).String())
	case *v1.Service:
		service, ok := obj.(*v1.Service)
		if !ok {
			return errors.New("type assertion failure")
		}
		if len(service.GetAnnotations()) == 0 {
			return ErrMissingAnnotations
		}
	}
	return nil
}

// GetBoolAnnotation extracts a boolean from an annotation or returns an error
func GetBoolAnnotation(name string, obj interface{}) (bool, error) {
	err := checkAnnotation(name, obj)
	if err != nil {
		return false, err
	}
	return serviceAnnotations(obj.(*v1.Service).GetAnnotations()).parseBool(name)
}

// GetOptionalBoolAnnotation returns the boolean value of the annotations, or
// the boolean zero value if not present, and a bool to indicate wether or not
// the value was present
func GetOptionalBoolAnnotation(name string, obj interface{}) (bool, bool) {
	value, err := GetBoolAnnotation(name, obj)
	if err != nil && !IsMissingAnnotations(err) {
		glog.Warningf("unexpected error reading annotation: %v", err)
		return false, false
	}
	return value, true
}

// GetStringAnnotation extracts a string from an annotation or returns an error
func GetStringAnnotation(name string, obj interface{}) (string, error) {
	err := checkAnnotation(name, obj)
	if err != nil {
		return "", err
	}
	return serviceAnnotations(obj.(*v1.Service).GetAnnotations()).parseString(name)
}

// GetOptionalStringAnnotation returns the string value of the annotations, or
// the empty string if not present, and a bool to indicate wether or not the
// value was present
func GetOptionalStringAnnotation(name string, obj interface{}) (string, bool) {
	value, err := GetStringAnnotation(name, obj)
	if err != nil && !IsMissingAnnotations(err) {
		glog.Warningf("unexpected error reading annotation: %v", err)
		return "", false
	}
	return value, true
}

// GetIntAnnotation extracts an int from an annotation or returns an error
func GetIntAnnotation(name string, obj interface{}) (int, error) {
	err := checkAnnotation(name, obj)
	if err != nil {
		return 0, err
	}
	return serviceAnnotations(obj.(*v1.Service).GetAnnotations()).parseInt(name)
}

// GetOptionalIntAnnotation returns the integer value of the annotations, or
// the int zero value if not present, and a bool to indicate wether or not the
// value was present
func GetOptionalIntAnnotation(name string, obj interface{}) (int, bool) {
	value, err := GetIntAnnotation(name, obj)
	if err != nil && !IsMissingAnnotations(err) {
		glog.Warningf("unexpected error reading annotation: %v", err)
		return 0, false
	}
	return value, true
}

// IsValid returns true if the given Service object specifies 'lbex' as the
// value to the loadbalancer.class annotation.
func IsValid(obj interface{}) bool {
	class, _ := GetOptionalStringAnnotation(LBEXClassKey, obj)
	return class == LBEXClassKeyValue
}
