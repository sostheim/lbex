package main

import (
	goflag "flag"
	"fmt"
	"time"

	"github.com/blang/semver"
	"github.com/golang/glog"
	flag "github.com/spf13/pflag"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api/unversioned"
	"k8s.io/client-go/pkg/util/wait"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// LbexMajorMinorPatch - semantic version string
var LbexMajorMinorPatch string

// LbexType - release type
var LbexType = "alpha"

// LbexGitCommit - git commit sha-1 hash
var LbexGitCommit string

var lbexCfg *config

func init() {
	go wait.Until(glog.Flush, 10*time.Second, wait.NeverStop)
	lbexCfg = newConfig()
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
	config, err := clientcmd.BuildConfigFromFlags("", *lbexCfg.kubeconfig)
	if err != nil {
		panic(err.Error())
	}
	return config
}

func byProxy() *rest.Config {
	glog.V(3).Infof("byProxy(): creating config")
	return &rest.Config{
		Host: *lbexCfg.proxy,
	}
}

func displayVersion() {
	semVer, err := semver.Make(LbexMajorMinorPatch + "-" + LbexType + "+git.sha." + LbexGitCommit)
	if err != nil {
		panic(err)
	}
	fmt.Println(semVer.String())
}

func main() {
	flag.CommandLine.AddGoFlagSet(goflag.CommandLine)
	flag.Parse()

	lbexCfg.flagSet = flag.CommandLine
	lbexCfg.envParse()

	// check for version flag, if present print veriosn and exit
	if *lbexCfg.version {
		displayVersion()
		return
	}
	// creates the config, in preference order, for:
	// 1 - the proxy URL, if present as an argument
	// 2 - kubeconfig, if present as an argument
	// 3 - otherwise assume execution on an in-cluster node
	//     note: this will fail with the appropriate error messages
	//           if not actually executing on a node in the cluster.
	var config *rest.Config
	if *lbexCfg.proxy != "" {
		config = byProxy()
	} else if *lbexCfg.kubeconfig != "" {
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
	lbex := newLbExController(clientset, lbexCfg)
	lbex.run()

	for {
		time.Sleep(15 * time.Second)
	}
}
