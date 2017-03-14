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
	"flag"
	"os"
	"time"

	"github.com/golang/glog"
	"github.com/spf13/pflag"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api/unversioned"
	"k8s.io/client-go/pkg/util/wait"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

var (
	resyncPeriod = 30 * time.Second
	flags        = pflag.NewFlagSet("", pflag.ExitOnError)
)

// List Watch (lw) Controller (lwc)
type lwController struct {
	controller  *cache.Controller
	store       cache.Store
	queue       *taskQueue
	syncHandler func(key string) error
	stopCh      chan struct{}
}

// External LB Controller (lbex)
type lbExController struct {
	client       *dynamic.Client
	clientset    *kubernetes.Clientset
	endpointsLWC *lwController
	servciesLWC  *lwController
	stopCh       chan struct{}
}

func init() {
	flag.Set("logtostderr", "true")
	flag.Parse()
	go wait.Until(glog.Flush, 10*time.Second, wait.NeverStop)
}

func startListWatches(lbex *lbExController) {

	lbex.servciesLWC = newServicesListWatchControllerForClientset(lbex.clientset)
	lbex.endpointsLWC = newEndpointsListWatchControllerForClientset(lbex.clientset)

	// run the controller goroutines
	go lbex.servciesLWC.controller.Run(wait.NeverStop)
	go lbex.endpointsLWC.controller.Run(wait.NeverStop)
}

func main() {

	glog.V(3).Infof("lbex.main(): starting")

	// Per https://github.com/kubernetes/kubernetes/issues/17162
	// Supress goflag's warnings spewing to logs.
	flags.AddGoFlagSet(flag.CommandLine)
	flags.Parse(os.Args)
	flag.CommandLine.Parse([]string{})

	// creates the in-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		panic(err.Error())
	}
	config.GroupVersion = &unversioned.GroupVersion{
		Group:   "",
		Version: "v1",
	}

	glog.V(3).Infof("lbex.main(): created config")

	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	// create dynamic client
	client, err := dynamic.NewClient(config)
	if err != nil {
		panic(err.Error())
	}

	glog.V(3).Infof("lbex.main(): created client")

	// create external loadbalancer controller struct
	lbex := &lbExController{
		client:    client,
		clientset: clientset,
		stopCh:    make(chan struct{}),
	}

	glog.V(3).Infof("lbex.main(): staring controllers")

	// services/endpoint controller
	startListWatches(lbex)

	for {
		time.Sleep(20 * time.Second)
	}
}
