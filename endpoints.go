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

	"k8s.io/client-go/pkg/api"
	"k8s.io/client-go/pkg/api/unversioned"
	"k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/pkg/fields"
	"k8s.io/client-go/tools/cache"
)

var (
	epAPIResource = unversioned.APIResource{Name: "endpoints", Namespaced: true, Kind: "endpoint"}
)

func newEndpointsListWatchController() *lwController {
	return &lwController{
		stopCh: make(chan struct{}),
	}
}

func newEndpointsListWatchControllerForClientset(lbex *lbExController) *lwController {

	lwc := newEndpointsListWatchController()

	//Setup an informer to call functions when the ListWatch changes
	listWatch := cache.NewListWatchFromClient(
		lbex.clientset.Core().RESTClient(), "endpoints", api.NamespaceAll, fields.Everything())

	eventHandler := cache.ResourceEventHandlerFuncs{
		AddFunc:    endpointCreatedFunc(lbex),
		DeleteFunc: endpointDeletedFunc(lbex),
		UpdateFunc: endpointUpdatedFunc(lbex),
	}

	lbex.endpointStore, lwc.controller = cache.NewInformer(listWatch, &v1.Endpoints{}, resyncPeriod, eventHandler)

	return lwc
}

func endpointCreatedFunc(lbex *lbExController) func(obj interface{}) {
	return func(obj interface{}) {
		if filterObject(obj) {
			glog.V(5).Infof("AddFunc: filtering endpoint object")
			return
		}
		glog.V(5).Infof("AddFunc: enqueuing endpoint object")
		lbex.endpointsQueue.Enqueue(obj)
	}
}

func endpointDeletedFunc(lbex *lbExController) func(obj interface{}) {
	return func(obj interface{}) {
		if filterObject(obj) {
			glog.V(5).Infof("DeleteFunc: filtering endpoint object")
			return
		}
		glog.V(5).Infof("DeleteFunc: enqueuing endpoint object")
		lbex.endpointsQueue.Enqueue(obj)
	}
}

func endpointUpdatedFunc(lbex *lbExController) func(obj, newObj interface{}) {
	return func(obj, newObj interface{}) {
		if filterObject(obj) {
			glog.V(5).Infof("UpdateFunc: filtering endpoint object")
			return
		}
		if !reflect.DeepEqual(obj, newObj) {
			glog.V(5).Infof("UpdateFunc: enqueuing unequal endpoint object")
			lbex.endpointsQueue.Enqueue(newObj)
		}
	}
}
