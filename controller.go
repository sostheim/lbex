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
	"time"

	"github.com/golang/glog"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

var (
	resyncPeriod = 30 * time.Second
)

// List Watch (lw) Controller (lwc)
type lwController struct {
	controller *cache.Controller
	stopCh     chan struct{}
}

// External LB Controller (lbex)
type lbExController struct {
	client    *dynamic.Client
	clientset *kubernetes.Clientset

	endpointsLWC  *lwController
	endpointStore cache.Store

	servciesLWC   *lwController
	servicesStore cache.Store

	stopCh chan struct{}
	queue  *TaskQueue
}

func newLbExController(client *dynamic.Client, clientset *kubernetes.Clientset) *lbExController {
	// create external loadbalancer controller struct
	lbexc := lbExController{
		client:    client,
		clientset: clientset,
		stopCh:    make(chan struct{}),
	}
	lbexc.queue = NewTaskQueue(lbexc.sync)
	return &lbexc
}

func (lbex *lbExController) sync(key string) {
	storeObj, exists, err := lbex.servicesStore.GetByKey(key)
	if err != nil {
		return
	}
	if !exists {
		glog.V(3).Infof("syncServices: unable to find services for key value: %s", key)
	} else {
		glog.V(3).Infof("syncServices: updating Services for %v", storeObj)
	}
	return
}
