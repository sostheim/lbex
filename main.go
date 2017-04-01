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

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api/unversioned"
	"k8s.io/client-go/pkg/util/wait"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	kubeconfig  = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	proxy       = flag.String("proxy", "", "kubctl proxy server running at the given url")
	serviceName = flag.String("service-name", "", `Proive LoadBalancing for the specified service.`)
)

func init() {
	go wait.Until(glog.Flush, 10*time.Second, wait.NeverStop)
}

func addGV(config *rest.Config) {
	config.ContentConfig.GroupVersion = &unversioned.GroupVersion{
		Group:   "",
		Version: "v1",
	}
}

func inCluster() *rest.Config {
	glog.V(3).Infof("inCluster(): creating config")
	config, err := rest.InClusterConfig()
	if err != nil {
		panic(err.Error())
	}
	return config
}

func external() *rest.Config {
	glog.V(3).Infof("external(): creating config")
	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		panic(err.Error())
	}
	return config
}

func byProxy() *rest.Config {
	glog.V(3).Infof("byProxy(): creating config")
	return &rest.Config{
		Host: *proxy,
	}
}

func main() {
	flag.CommandLine.AddGoFlagSet(goflag.CommandLine)
	flag.Parse()

	// creates the config, in preference order, for:
	// 1 - the proxy URL, if present as an argument
	// 2 - kubeconfig, if present as an arguemtn
	// 3 - otherwise assume execution on an in-cluster node
	//     note: this will fail with the appropriate error messages
	//           if not actually executing on a node in the cluster.
	var config *rest.Config
	if *proxy != "" {
		config = byProxy()
	} else if *kubeconfig != "" {
		config = external()
	} else {
		config = inCluster()
	}
	addGV(config)

	// creates a clientset
	glog.V(3).Infof("main(): create clientset from config")
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	// services/endpoint controller
	glog.V(3).Infof("main(): staring controllers")
	lbex := newLbExController(clientset, serviceName)
	lbex.run()

	for {
		time.Sleep(15 * time.Second)
	}
}
