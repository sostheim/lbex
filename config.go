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

	flag "github.com/spf13/pflag"
)

type config struct {
	kubeconfig      *string
	proxy           *string
	serviceName     *string
	servicePool     *string
	strictAffinity  *bool
	antiAffinity    *bool
	version         *bool
	healthCheck     *bool
	healthCheckPort *int
	requirePort     *bool
}

func newConfig() *config {
	return &config{
		kubeconfig:      flag.String("kubeconfig", "", "absolute path to the kubeconfig file"),
		proxy:           flag.String("proxy", "", "kubctl proxy server running at the given url"),
		serviceName:     flag.String("service-name", "", "provide load balancing for the service-name - ONLY"),
		servicePool:     flag.String("service-pool", "", "provide load balancing for services in --service-pool"),
		strictAffinity:  flag.Bool("strict-affinity", false, "provide load balancing for services in --service-pool ONLY"),
		antiAffinity:    flag.Bool("anti-affinity", false, "do not provide load balancing for services in --service-pool"),
		version:         flag.Bool("version", false, "display version info and exit"),
		healthCheck:     flag.Bool("health-check", true, "enable health checking for LBEX"),
		healthCheckPort: flag.Int("health-port", 7331, "health check service port"),
		requirePort:     flag.Bool("require-port", true, "makes the Service Specification annotation \"loadbalancer.lbex/port\" required"),
	}
}

func (cfg *config) String() string {
	return fmt.Sprintf("kubeconfig: %s, proxy: %s, service-name: %s, service-pool: %s, strict-affinity: %t, "+
		"anti-affinity: %t, health-check: %t, health-check-port: %d, require-port: %t",
		*cfg.kubeconfig, *cfg.proxy, *cfg.serviceName, *cfg.servicePool, *cfg.strictAffinity,
		*cfg.antiAffinity, *cfg.healthCheck, *cfg.healthCheckPort, *cfg.requirePort)
}
