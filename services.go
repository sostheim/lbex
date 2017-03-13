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
	svcApiResource = unversioned.APIResource{Name: "services", Namespaced: true, Kind: "service"}
)

func newServicesListWatchControllerForClient(client *dynamic.Client) *lwController {

	resyncPeriod := 30 * time.Second

	//Setup an informer to call functions when the watchlist changes
	listWatch := &cache.ListWatch{
		ListFunc: func(options api.ListOptions) (runtime.Object, error) {
			return client.Resource(&svcApiResource, api.NamespaceAll).List(&options)
		},
		WatchFunc: func(options api.ListOptions) (watch.Interface, error) {
			return client.Resource(&svcApiResource, api.NamespaceAll).Watch(&options)
		},
	}
	eventHandlers := cache.ResourceEventHandlerFuncs{
		AddFunc:    serviceCreated,
		UpdateFunc: serviceUpdated,
		DeleteFunc: serviceDeleted,
	}

	lwc := lwController{
		stopCh: make(chan struct{}),
	}

	lwc.queue = newTaskQueue(lwc.syncServices)

	lwc.store, lwc.controller = cache.NewInformer(
		listWatch,
		&api.Service{},
		resyncPeriod,
		eventHandlers,
	)

	return &lwc
}

func serviceCreated(obj interface{}) {
	if service, ok := obj.(*api.Service); ok {
		glog.V(3).Infof("Service created: " + service.ObjectMeta.Name)
	} else {
		glog.V(3).Infof("serviceCreate: obj interface{} not of type api.service")
	}
}

func serviceDeleted(obj interface{}) {
	if service, ok := obj.(*api.Service); ok {
		glog.V(3).Infof("Service deleted: " + service.ObjectMeta.Name)
	} else {
		glog.V(3).Infof("serviceDeleted: obj interface{} not of type api.service")
	}
}

func serviceUpdated(obj, newObj interface{}) {
	if service, ok := obj.(*api.Service); ok {
		glog.V(3).Infof("Service updated: " + service.ObjectMeta.Name)
	} else {
		glog.V(3).Infof("serviceUpdated: obj interface{} not of type api.service")
	}
}

/*
func serviceListFunc(clientset *kubernetes.Clientset, namespace string) func(api.ListOptions) (runtime.Object, error) {
	return func(options api.ListOptions) (runtime.Object, error) {
		return clientset.CoreV1().Services(namespace).List(options)
	}
}

func serviceWatchFunc(clientset *kubernetes.Clientset, namespace string) func(options api.ListOptions) (watch.Interface, error) {
	return func(options api.ListOptions) (watch.Interface, error) {
		return clientset.CoreV1().Services(namespace).Watch(options)
	}
}
*/

func (lwc *lwController) syncServices(key string) {
	//glog.V(3).Infof("Syncing services %v", key)

	obj, exists, err := lwc.store.GetByKey(key)
	if err != nil {
		lwc.queue.requeue(key, err)
		return
	}
	if !exists {
		log.Print(fmt.Sprintf("Unable to find services for key value: %s", key))
	} else {
		log.Print(fmt.Sprintf("Updating Services for %v", obj))
	}
}
