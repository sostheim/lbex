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
	"reflect"

	"github.com/golang/glog"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api"
	"k8s.io/client-go/pkg/api/unversioned"
	"k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/pkg/fields"
	"k8s.io/client-go/pkg/runtime"
	"k8s.io/client-go/pkg/watch"
	"k8s.io/client-go/tools/cache"
)

var (
	epAPIResource = unversioned.APIResource{Name: "endpoints", Namespaced: true, Kind: "endpoint"}
)

func newEndpointsListWatchController() *lwController {
	lwc := lwController{
		stopCh: make(chan struct{}),
	}
	return &lwc
}

func newEndpointsListWatchControllerForClient(lbex *lbExController) *lwController {

	lwc := newEndpointsListWatchController()

	//Setup an informer to call functions when the watchlist changes
	listWatch := &cache.ListWatch{
		ListFunc:  clientEndpointsListFunc(lbex.client, api.NamespaceAll),
		WatchFunc: clientEndpointsWatchFunc(lbex.client, api.NamespaceAll),
	}
	eventHandler := cache.ResourceEventHandlerFuncs{
		AddFunc:    endpointCreated,
		DeleteFunc: endpointDeleted,
		UpdateFunc: endpointUpdated,
	}

	lbex.endpointStore, lwc.controller = cache.NewInformer(listWatch, &api.Endpoints{}, resyncPeriod, eventHandler)

	return lwc
}

func newEndpointsListWatchControllerForClientset(lbex *lbExController) *lwController {

	lwc := newEndpointsListWatchController()

	//Setup an informer to call functions when the watchlist changes
	listWatch := cache.NewListWatchFromClient(
		lbex.clientset.Core().RESTClient(), "endpoints", api.NamespaceAll, fields.Everything())

	eventHandler := cache.ResourceEventHandlerFuncs{
		//AddFunc:    endpointCreated,
		//DeleteFunc: endpointDeleted,
		//UpdateFunc: endpointUpdated,
		AddFunc: func(obj interface{}) {
			lbex.queue.Enqueue(obj)
		},
		DeleteFunc: func(obj interface{}) {
			lbex.queue.Enqueue(obj)
		},
		UpdateFunc: func(old, cur interface{}) {
			if !reflect.DeepEqual(old, cur) {
				lbex.queue.Enqueue(cur)
			}
		},
	}

	lbex.endpointStore, lwc.controller = cache.NewInformer(listWatch, &v1.Endpoints{}, resyncPeriod, eventHandler)

	return lwc
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

func clientEndpointsListFunc(client *dynamic.Client, namespace string) func(api.ListOptions) (runtime.Object, error) {
	return func(options api.ListOptions) (runtime.Object, error) {
		return client.Resource(&epAPIResource, api.NamespaceAll).List(&options)
	}
}

func clientEndpointsWatchFunc(client *dynamic.Client, namespace string) func(options api.ListOptions) (watch.Interface, error) {
	return func(options api.ListOptions) (watch.Interface, error) {
		return client.Resource(&epAPIResource, api.NamespaceAll).Watch(&options)
	}
}

func clientsetEndpointsListFunc(client *kubernetes.Clientset, namespace string) func(v1.ListOptions) (runtime.Object, error) {
	return func(options v1.ListOptions) (runtime.Object, error) {
		return client.CoreV1().Endpoints(namespace).List(options)
	}
}

func clientsetEndpointsWatchFunc(client *kubernetes.Clientset, namespace string) func(options v1.ListOptions) (watch.Interface, error) {
	return func(options v1.ListOptions) (watch.Interface, error) {
		return client.CoreV1().Endpoints(namespace).Watch(options)
	}
}
