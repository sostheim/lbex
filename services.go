package main

import (
	"reflect"
	"strings"

	"github.com/golang/glog"

	"k8s.io/client-go/pkg/api"
	"k8s.io/client-go/pkg/api/unversioned"
	v1 "k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/pkg/fields"
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

func newServicesListWatchControllerForClientset(lbex *lbExController) *lwController {

	lwc := newServicesListWatchController()

	//Setup an informer to call functions when the ListWatch changes
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

func filterObject(obj interface{}) bool {
	// obj can be filtered for either a: type conversion failure,
	// b: namespace is 'kube-system/' - which we don't handle.
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		glog.V(5).Infof("filterObject: DeletionHandlingMetaNamespaceKeyFunc(): err: %v", err)
		return true
	}
	glog.V(5).Infof("filterObject: return %s has prefix 'kube-system/'", key)
	return strings.HasPrefix(key, "kube-system/")
}

func serviceCreatedFunc(lbex *lbExController) func(obj interface{}) {
	return func(obj interface{}) {
		if filterObject(obj) {
			glog.V(5).Infof("AddFunc: filtering out service object")
			return
		}
		glog.V(5).Infof("AddFunc: enqueuing service object")
		lbex.servicesQueue.Enqueue(obj)
	}
}

func serviceDeletedFunc(lbex *lbExController) func(obj interface{}) {
	return func(obj interface{}) {
		if filterObject(obj) {
			glog.V(5).Infof("DeleteFunc: filtering out service object")
			return
		}
		glog.V(5).Infof("DeleteFunc: enqueuing service object")
		lbex.servicesQueue.Enqueue(obj)
	}
}
func serviceUpdatedFunc(lbex *lbExController) func(obj, newObj interface{}) {
	return func(obj, newObj interface{}) {
		if filterObject(obj) {
			glog.V(5).Infof("UpdateFunc: filtering out service object")
			return
		}
		if !reflect.DeepEqual(obj, newObj) {
			glog.V(5).Infof("UpdateFunc: enqueuing unequal service object")
			lbex.servicesQueue.Enqueue(newObj)
		}
	}
}
