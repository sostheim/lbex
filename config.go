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
	"os"
	"strings"

	"github.com/golang/glog"
	flag "github.com/spf13/pflag"
)

type config struct {
	flagSet         *flag.FlagSet
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

var envSupport = map[string]bool{
	"kubeconfig":      true,
	"proxy":           true,
	"service-name":    true,
	"service-pool":    true,
	"strict-affinity": true,
	"anti-affinity":   true,
	"version":         false,
	"health-check":    true,
	"health-port":     true,
	"require-port":    true,
}

func variableName(name string) string {
	return "LBEX_" + strings.ToUpper(strings.Replace(name, "-", "_", -1))
}

// Just like Flags.Parse() except we try to get recognized values for the valid
// set of flags from environment variables.  We choose to use the environment
// value if 1) the value hasen't already been set as command line flags and the
// flas is a member of the supported set (see map defined above).
func (cfg *config) envParse() error {
	var err error

	alreadySet := make(map[string]bool)
	cfg.flagSet.Visit(func(f *flag.Flag) {
		if envSupport[f.Name] {
			alreadySet[variableName(f.Name)] = true
		}
	})

	usedEnvKey := make(map[string]bool)
	cfg.flagSet.VisitAll(func(f *flag.Flag) {
		if envSupport[f.Name] {
			key := variableName(f.Name)
			glog.V(4).Infof("supported key: %v", key)
			if !alreadySet[key] {
				val := os.Getenv(key)
				if val != "" {
					usedEnvKey[key] = true
					if serr := cfg.flagSet.Set(f.Name, val); serr != nil {
						err = fmt.Errorf("invalid value %q for %s: %v", val, key, serr)
					}
					glog.V(3).Infof("recognized and used environment variable %s=%s", key, val)
				}
			}
		}
	})

	for _, env := range os.Environ() {
		kv := strings.SplitN(env, "=", 2)
		if len(kv) != 2 {
			glog.Warningf("found invalid env %s", env)
		}
		if usedEnvKey[kv[0]] {
			continue
		}
		if alreadySet[kv[0]] {
			glog.V(3).Infof("recognized environment variable %s, but unused: superseeded by command line flag ", kv[0])
			continue
		}
		if strings.HasPrefix(env, "LBEX_") {
			glog.Warningf("unrecognized environment variable %s", env)
		}
	}

	return err
}
