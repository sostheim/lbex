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
	"fmt"

	"k8s.io/client-go/pkg/api"
)

var (
	lbAPIPort      = 8081
	lbAlgorithmKey = "lbex/lb.algorithm"
	lbHostKey      = "lbex/lb.host"
)

// getTargetPort returns the numeric value of TargetPort
func getTargetPort(servicePort *api.ServicePort) int {
	return servicePort.TargetPort.IntValue()
}

// convenience type name modifications for lb rules.
func getServiceNameForLBRule(s *api.Service, servicePort int) string {
	if servicePort == 80 {
		return s.Name
	}
	return fmt.Sprintf("%v:%v", s.Name, servicePort)
}

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

type serviceAnnotations map[string]string

func (s serviceAnnotations) getAlgorithm() (string, bool) {
	val, ok := s[lbAlgorithmKey]
	return val, ok
}

func (s serviceAnnotations) getHost() (string, bool) {
	val, ok := s[lbHostKey]
	return val, ok
}