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
	goflag "flag"
	"time"

	"github.com/golang/glog"
	flag "github.com/spf13/pflag"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api/unversioned"
	"k8s.io/client-go/pkg/util/wait"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	proxy      = flag.String("proxy", "", "kubctl proxy server running at the given url")
)

func init() {
	go wait.Until(glog.Flush, 10*time.Second, wait.NeverStop)
}

func startListWatches(lbex *lbExController) {

	lbex.servciesLWC = newServicesListWatchControllerForClientset(lbex)
	lbex.endpointsLWC = newEndpointsListWatchControllerForClientset(lbex)

	// run the controller goroutines
	go lbex.servciesLWC.controller.Run(lbex.stopCh)
	go lbex.endpointsLWC.controller.Run(lbex.stopCh)

	go lbex.queue.Run(5*time.Second, lbex.stopCh)
}

func addGV(config *rest.Config) {
	config.ContentConfig.GroupVersion = &unversioned.GroupVersion{
		Group:   "",
		Version: "v1",
	}
}

func inCluster() *rest.Config {
	glog.V(3).Infof("inCluster(): creating config")

	// creates the in-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		panic(err.Error())
	}
	return config
}

func external() *rest.Config {
	glog.V(3).Infof("external(): creating config")
	// uses the current context in kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		panic(err.Error())
	}
	return config
}

func byProxy() *rest.Config {
	glog.V(3).Infof("byProxy(): creating config")
	rc := &rest.Config{
		Host: *proxy,
	}
	return rc
}

func main() {
	glog.V(3).Infof("main(): starting")

	flag.CommandLine.AddGoFlagSet(goflag.CommandLine)
	flag.Parse()

	glog.V(3).Infof("main(): creating new config")

	var config *rest.Config
	// creates the in-cluster config
	if *proxy != "" {
		config = byProxy()
	} else if *kubeconfig != "" {
		config = external()
	} else {
		config = inCluster()
	}

	glog.V(3).Infof("main(): create clientset from config")

	addGV(config)

	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	glog.V(3).Infof("main(): create client from config")

	// create dynamic client
	client, err := dynamic.NewClient(config)
	if err != nil {
		panic(err.Error())
	}

	// create external loadbalancer controller struct
	lbex := newLbExController(client, clientset)

	glog.V(3).Infof("main(): staring controllers")

	// services/endpoint controller
	startListWatches(lbex)

	for {
		time.Sleep(20 * time.Second)
	}
}
