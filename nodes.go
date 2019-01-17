package main

import (
	"reflect"

	"github.com/golang/glog"

	"k8s.io/client-go/pkg/api"
	"k8s.io/client-go/pkg/api/unversioned"
	v1 "k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/pkg/fields"
	"k8s.io/client-go/tools/cache"
)

var (
	nodeAPIResource = unversioned.APIResource{Name: "nodes", Namespaced: false, Kind: "node"}
)

func newNodesListWatchController() *lwController {
	return &lwController{
		stopCh: make(chan struct{}),
	}
}

func newNodesListWatchControllerForClientset(lbex *lbExController) *lwController {

	lwc := newNodesListWatchController()

	//Setup an informer to call functions when the ListWatch changes
	listWatch := cache.NewListWatchFromClient(
		lbex.clientset.Core().RESTClient(), "Nodes", api.NamespaceAll, fields.Everything())

	eventHandler := cache.ResourceEventHandlerFuncs{
		AddFunc:    nodeCreatedFunc(lbex),
		DeleteFunc: nodeDeletedFunc(lbex),
		UpdateFunc: nodeUpdatedFunc(lbex),
	}

	lbex.nodesStore, lwc.controller = cache.NewInformer(listWatch, &v1.Node{}, resyncPeriod, eventHandler)

	return lwc
}

// filterNode returns true if the node should be filtered, false otherwise
func filterNode(obj interface{}) bool {
	// obj can be filtered for either a: type conversion failure
	// *Removed Criteria* b: node is marked as scheduleable for pod placement.
	// checking scheduleable makes it impossible to remove a node that
	// has been newly marked as unschduleable.
	_, ok := obj.(*v1.Node)
	return !ok
}

func nodeCreatedFunc(lbex *lbExController) func(obj interface{}) {
	return func(obj interface{}) {
		if filterNode(obj) {
			glog.V(5).Infof("AddFunc: filtering out node object")
			return
		}
		glog.V(5).Infof("AddFunc: enqueuing node object")
		lbex.nodesQueue.Enqueue(obj)
	}
}

func nodeDeletedFunc(lbex *lbExController) func(obj interface{}) {
	return func(obj interface{}) {
		if filterNode(obj) {
			glog.V(5).Infof("DeleteFunc: filtering out node object")
			return
		}
		glog.V(5).Infof("DeleteFunc: enqueuing node object")
		lbex.nodesQueue.Enqueue(obj)
	}
}

func nodeUpdateEqual(old, new *v1.Node) bool {
	// Things we dont care about:
	// Data that should be static for a given node:
	// - node.metadata.creationtimestamp
	// - node.metadata.name
	// - node.metadata.selflink
	// - node.metadata.resourceversion
	// - node.metadata.UID
	// - node.status.allocateable
	// - node.status.capacity
	// - node.status.nodeinfo
	// Data that varies freqently, but doesn't affect our ability to use the node:
	// - node.status.conditions <--- constantly changing timestamps for health checks
	// - node.status.images <--- chanes every time a new image is pulled

	return reflect.DeepEqual(old.GetAnnotations(), new.GetAnnotations()) &&
		reflect.DeepEqual(old.GetLabels(), new.GetLabels()) &&
		reflect.DeepEqual(old.Spec, new.Spec) &&
		reflect.DeepEqual(old.Status.Addresses, new.Status.Addresses) &&
		reflect.DeepEqual(old.Status.DaemonEndpoints, new.Status.DaemonEndpoints)
}

func nodeUpdatedFunc(lbex *lbExController) func(obj, newObj interface{}) {
	return func(obj, newObj interface{}) {
		if filterNode(obj) || filterNode(newObj) {
			glog.V(5).Infof("UpdateFunc: filtering out node object")
			return
		}
		if !nodeUpdateEqual(obj.(*v1.Node), newObj.(*v1.Node)) {
			glog.V(5).Infof("UpdateFunc: enqueuing unequal node object")
			lbex.nodesQueue.Enqueue(newObj)
		}
	}
}
