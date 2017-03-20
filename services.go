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
	svcAPIResource = unversioned.APIResource{Name: "services", Namespaced: true, Kind: "service"}
)

func newServicesListWatchController() *lwController {
	return &lwController{
		stopCh: make(chan struct{}),
	}
}

func newServicesListWatchControllerForClient(lbex *lbExController) *lwController {

	lwc := newServicesListWatchController()

	//Setup an informer to call functions when the watchlist changes
	listWatch := &cache.ListWatch{
		ListFunc:  clientServicesListFunc(lbex.client, api.NamespaceAll),
		WatchFunc: clientServicesWatchFunc(lbex.client, api.NamespaceAll),
	}
	eventHandlers := cache.ResourceEventHandlerFuncs{
		AddFunc:    serviceCreatedFunc(lbex),
		UpdateFunc: serviceUpdatedFunc(lbex),
		DeleteFunc: serviceDeletedFunc(lbex),
	}

	lbex.servicesStore, lwc.controller = cache.NewInformer(listWatch, &api.Service{}, resyncPeriod, eventHandlers)

	return lwc
}

func newServicesListWatchControllerForClientset(lbex *lbExController) *lwController {

	lwc := newServicesListWatchController()

	//Setup an informer to call functions when the watchlist changes
	listWatch := cache.NewListWatchFromClient(
		lbex.clientset.Core().RESTClient(), "services", api.NamespaceAll, fields.Everything())

	eventHandler := cache.ResourceEventHandlerFuncs{
		AddFunc:    serviceCreatedFunc(lbex),
		DeleteFunc: serviceDeletedFunc(lbex),
		UpdateFunc: serviceUpdatedFunc(lbex),
	}

	lbex.servicesStore, lwc.controller = cache.NewInformer(listWatch, &v1.Service{}, resyncPeriod, eventHandler)

	return lwc
}

func serviceCreatedFunc(lbex *lbExController) func(obj interface{}) {
	return func(obj interface{}) {
		glog.V(3).Infof("AddFunc: enqueuing service object")
		lbex.queue.Enqueue(obj)
	}
}

func serviceDeletedFunc(lbex *lbExController) func(obj interface{}) {
	return func(obj interface{}) {
		glog.V(3).Infof("DeleteFunc: enqueuing service object")
		lbex.queue.Enqueue(obj)
	}
}
func serviceUpdatedFunc(lbex *lbExController) func(obj, newObj interface{}) {
	return func(obj, newObj interface{}) {
		if !reflect.DeepEqual(obj, newObj) {
			glog.V(3).Infof("UpdateFunc: enqueuing unequal service object")
			lbex.queue.Enqueue(newObj)
		}
	}
}

func clientServicesListFunc(client *dynamic.Client, namespace string) func(api.ListOptions) (runtime.Object, error) {
	return func(options api.ListOptions) (runtime.Object, error) {
		return client.Resource(&svcAPIResource, api.NamespaceAll).List(&options)
	}
}

func clientServicesWatchFunc(client *dynamic.Client, namespace string) func(options api.ListOptions) (watch.Interface, error) {
	return func(options api.ListOptions) (watch.Interface, error) {
		return client.Resource(&svcAPIResource, api.NamespaceAll).Watch(&options)
	}
}

func clientsetServicesListFunc(client *kubernetes.Clientset, namespace string) func(v1.ListOptions) (runtime.Object, error) {
	return func(options v1.ListOptions) (runtime.Object, error) {
		return client.CoreV1().Endpoints(namespace).List(options)
	}
}

func clientsetServicesWatchFunc(client *kubernetes.Clientset, namespace string) func(options v1.ListOptions) (watch.Interface, error) {
	return func(options v1.ListOptions) (watch.Interface, error) {
		return client.CoreV1().Endpoints(namespace).Watch(options)
	}
}

// GetServiceEndpoints returns the endpoints for the specified service name / namesapce.
func (lbex *lbExController) GetServiceEndpoints(service *api.Service) (endpoints api.Endpoints, err error) {
	for _, svc := range lbex.servicesStore.List() {
		endpoints = *svc.(*api.Endpoints)
		if service.Name == endpoints.Name && service.Namespace == endpoints.Namespace {
			return endpoints, nil
		}
	}
	err = fmt.Errorf("could not find endpoints for service: %v", service.Name)
	return
}
