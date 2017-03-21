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
	secAPIResource = unversioned.APIResource{Name: "secrets", Namespaced: true, Kind: "secret"}
)

func newSecretsListWatchController() *lwController {
	return &lwController{
		stopCh: make(chan struct{}),
	}
}

func newSecretsListWatchControllerForClient(lbex *lbExController) *lwController {

	lwc := newSecretsListWatchController()

	//Setup an informer to call functions when the watchlist changes
	listWatch := &cache.ListWatch{
		ListFunc:  clientSecretsListFunc(lbex.client, api.NamespaceAll),
		WatchFunc: clientSecretsWatchFunc(lbex.client, api.NamespaceAll),
	}
	eventHandlers := cache.ResourceEventHandlerFuncs{
		AddFunc:    secretCreatedFunc(lbex),
		UpdateFunc: secretUpdatedFunc(lbex),
		DeleteFunc: secretDeletedFunc(lbex),
	}

	lbex.secretsStore, lwc.controller = cache.NewInformer(listWatch, &api.Secret{}, resyncPeriod, eventHandlers)

	return lwc
}

func newSecretsListWatchControllerForClientset(lbex *lbExController) *lwController {

	lwc := newSecretsListWatchController()

	//Setup an informer to call functions when the watchlist changes
	listWatch := cache.NewListWatchFromClient(
		lbex.clientset.Core().RESTClient(), "secrets", api.NamespaceAll, fields.Everything())

	eventHandler := cache.ResourceEventHandlerFuncs{
		AddFunc:    secretCreatedFunc(lbex),
		DeleteFunc: secretDeletedFunc(lbex),
		UpdateFunc: secretUpdatedFunc(lbex),
	}

	lbex.secretsStore, lwc.controller = cache.NewInformer(listWatch, &v1.Secret{}, resyncPeriod, eventHandler)

	return lwc
}

func secretCreatedFunc(lbex *lbExController) func(obj interface{}) {
	return func(obj interface{}) {
		glog.V(3).Infof("AddFunc: enqueuing secret object")
		lbex.queue.Enqueue(obj)
	}
}

func secretDeletedFunc(lbex *lbExController) func(obj interface{}) {
	return func(obj interface{}) {
		glog.V(3).Infof("DeleteFunc: enqueuing secret object")
		lbex.queue.Enqueue(obj)
	}
}
func secretUpdatedFunc(lbex *lbExController) func(obj, newObj interface{}) {
	return func(obj, newObj interface{}) {
		if !reflect.DeepEqual(obj, newObj) {
			glog.V(3).Infof("UpdateFunc: enqueuing unequal secret object")
			lbex.queue.Enqueue(newObj)
		}
	}
}

func clientSecretsListFunc(client *dynamic.Client, namespace string) func(api.ListOptions) (runtime.Object, error) {
	return func(options api.ListOptions) (runtime.Object, error) {
		return client.Resource(&secAPIResource, api.NamespaceAll).List(&options)
	}
}

func clientSecretsWatchFunc(client *dynamic.Client, namespace string) func(options api.ListOptions) (watch.Interface, error) {
	return func(options api.ListOptions) (watch.Interface, error) {
		return client.Resource(&secAPIResource, api.NamespaceAll).Watch(&options)
	}
}

func clientsetSecretsListFunc(client *kubernetes.Clientset, namespace string) func(v1.ListOptions) (runtime.Object, error) {
	return func(options v1.ListOptions) (runtime.Object, error) {
		return client.CoreV1().Secrets(namespace).List(options)
	}
}

func clientsetSecretsWatchFunc(client *kubernetes.Clientset, namespace string) func(options v1.ListOptions) (watch.Interface, error) {
	return func(options v1.ListOptions) (watch.Interface, error) {
		return client.CoreV1().Secrets(namespace).Watch(options)
	}
}
