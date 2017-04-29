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

import flag "github.com/spf13/pflag"

type config struct {
	kubeconfig      *string
	proxy           *string
	serviceName     *string
	servicePool     *string
	version         *bool
	healthCheck     *bool
	healthCheckPort *int
}

func newConfig() *config {
	return &config{
		kubeconfig:      flag.String("kubeconfig", "", "absolute path to the kubeconfig file"),
		proxy:           flag.String("proxy", "", "kubctl proxy server running at the given url"),
		serviceName:     flag.String("service-name", "", "Provide load balancing for the specified service - ONLY."),
		servicePool:     flag.String("service-pool", "", "Provide load balancing for services in the specified pool, and services for which no pool is specified."),
		version:         flag.Bool("version", false, "Display version info"),
		healthCheck:     flag.Bool("health-check", true, "Enable health checking for LBEX (default true)"),
		healthCheckPort: flag.Int("health-port", 7331, "health check service port (default 7331)"),
	}
}
