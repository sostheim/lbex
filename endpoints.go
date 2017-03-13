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
	"log"
	"time"

	"github.com/golang/glog"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/pkg/api"
	"k8s.io/client-go/pkg/api/unversioned"
	"k8s.io/client-go/pkg/runtime"
	"k8s.io/client-go/pkg/watch"
	"k8s.io/client-go/tools/cache"
)

var (
	epApiResource = unversioned.APIResource{Name: "endpoints", Namespaced: true, Kind: "endpoint"}
)

func newEndpointListWatchControllerForClient(client *dynamic.Client) *lwController {
	resyncPeriod := 30 * time.Second

	//Setup an informer to call functions when the watchlist changes
	listWatch := &cache.ListWatch{
		ListFunc: func(options api.ListOptions) (runtime.Object, error) {
			return client.Resource(&epApiResource, api.NamespaceAll).List(&options)
		},
		WatchFunc: func(options api.ListOptions) (watch.Interface, error) {
			return client.Resource(&epApiResource, api.NamespaceAll).Watch(&options)
		},
	}
	eventHandler := cache.ResourceEventHandlerFuncs{
		AddFunc:    endpointCreated,
		DeleteFunc: endpointDeleted,
		UpdateFunc: endpointUpdated,
	}

	lwc := lwController{
		stopCh: make(chan struct{}),
	}

	lwc.queue = newTaskQueue(lwc.syncEndpoints)

	lwc.store, lwc.controller = cache.NewInformer(
		listWatch,
		&api.Endpoints{},
		resyncPeriod,
		eventHandler,
	)

	return &lwc
}

func endpointCreated(obj interface{}) {
	if endpoint, ok := obj.(*api.Endpoints); ok {
		glog.V(3).Infof("Endpoint created: " + endpoint.ObjectMeta.Name)
	} else {
		glog.V(3).Infof("endpointCreated: obj interface{} not of type api.Endpoint")
	}
}

func endpointDeleted(obj interface{}) {
	if endpoint, ok := obj.(*api.Endpoints); ok {
		glog.V(3).Infof("Endpoint deleted: " + endpoint.ObjectMeta.Name)
	} else {
		glog.V(3).Infof("endpointDeleted: obj interface{} not of type api.Endpoint")
	}
}

func endpointUpdated(obj, newObj interface{}) {
	if endpoint, ok := obj.(*api.Endpoints); ok {
		glog.V(3).Infof("Endpoint updated: " + endpoint.ObjectMeta.Name)
	} else {
		glog.V(3).Infof("endpointUpdated: obj interface{} not of type api.Endpoint")
	}
}

func (lwc *lwController) syncEndpoints(key string) {
	//glog.V(3).Infof("Syncing endpoints %v", key)

	obj, exists, err := lwc.store.GetByKey(key)
	if err != nil {
		lwc.queue.requeue(key, err)
		return
	}
	if !exists {
		log.Print(fmt.Sprintf("Unable to find endpoint for key value: %s", key))
	} else {
		log.Print(fmt.Sprintf("Updating Endpoint for %v", obj))
	}
}
