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
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/golang/glog"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api"
	"k8s.io/client-go/pkg/util/intstr"
	"k8s.io/client-go/tools/cache"
)

var (
	resyncPeriod        = 30 * time.Second
	supportedAlgorithms = []string{"roundrobin", "leastconn"}
	defaultAlgorithm    = string("roundrobin")
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

	// The service to provide load balancing for, or "all" if empty
	service string
}

func newLbExController(client *dynamic.Client, clientset *kubernetes.Clientset, service *string) *lbExController {
	// create external loadbalancer controller struct
	lbexc := lbExController{
		client:    client,
		clientset: clientset,
		stopCh:    make(chan struct{}),
		service:   *service,
	}
	lbexc.queue = NewTaskQueue(lbexc.sync)
	lbexc.servciesLWC = newServicesListWatchControllerForClientset(&lbexc)
	lbexc.endpointsLWC = newEndpointsListWatchControllerForClientset(&lbexc)

	return &lbexc
}

func (lbex *lbExController) sync(obj interface{}) error {

	if lbex.queue.IsShuttingDown() {
		return nil
	}

	key, ok := obj.(string)
	if !ok {
		return errors.New("Invalid conversion from object any to string for key")
	}

	storeObj, exists, err := lbex.servicesStore.GetByKey(key)
	if err != nil {
		return err
	} else if exists {
		glog.V(3).Infof("sync: updating services for key: %s", key)
		glog.V(4).Infof("sync: updating services object %v", storeObj)
	} else {
		// TODO: this check needs to be outside the else condition, or have a
		// key that is guranteed to be unique from the service key.  Otherwise
		// endpoint objects will never get processed.
		storeObj, exists, err = lbex.endpointStore.GetByKey(key)
		if err != nil {
			return err
		} else if exists {
			glog.V(3).Infof("sync: updating endpoints for key %s", key)
			glog.V(4).Infof("sync: updating endpoint object %v", storeObj)
		} else {
			glog.V(3).Infof("sync: unable to find services or endpoint object for key value: %s", key)
		}
	}
	return nil
}

// getEndpoints returns a list of <endpoint ip>:<port> for a given service/target port combination.
func (lbex *lbExController) getEndpoints(service *api.Service, servicePort *api.ServicePort) (endpoints []string) {
	svcEndpoints, err := lbex.GetServiceEndpoints(service)
	if err != nil {
		return
	}

	// The intent here is to create a union of all subsets that match a targetPort.
	// We know the endpoint already matches the service, so all pod ips that have
	// the target port are capable of service traffic for it.
	for _, subsets := range svcEndpoints.Subsets {
		for _, epPort := range subsets.Ports {
			var targetPort int
			switch servicePort.TargetPort.Type {
			case intstr.Int:
				if epPort.Port == int32(getTargetPort(servicePort)) {
					targetPort = int(epPort.Port)
				}
			case intstr.String:
				if epPort.Name == servicePort.TargetPort.StrVal {
					targetPort = int(epPort.Port)
				}
			}
			if targetPort == 0 {
				continue
			}
			for _, epAddress := range subsets.Addresses {
				endpoints = append(endpoints, fmt.Sprintf("%v:%v", epAddress.IP, targetPort))
			}
		}
	}
	return
}

// getServices returns a list of services and their endpoints.
func (lbex *lbExController) getServices() (tcpServices []Service, udpServices []Service) {
	ep := []string{}
	objects := lbex.servicesStore.List()
	for _, obj := range objects {
		service, ok := obj.(*api.Service)
		if !ok {
			continue
		}
		if service.Spec.Type == api.ServiceTypeLoadBalancer {
			glog.V(3).Infof("service: %s has type: LoadBalancer - skipping", service.Name)
			continue
		}
		for _, servicePort := range service.Spec.Ports {
			// TODO: headless services?
			sName := service.Name
			if lbex.service != "" && lbex.service != sName {
				glog.Infof("Ignoring non-matching service: %s:%+d", sName, servicePort)
				continue
			}

			ep = lbex.getEndpoints(service, &servicePort)
			if len(ep) == 0 {
				glog.Infof("No endpoints found for service %v, port %+v",
					sName, servicePort)
				continue
			}
			newSvc := Service{
				Name:        getServiceNameForLBRule(service, int(servicePort.Port)),
				Ep:          ep,
				BackendPort: getTargetPort(&servicePort),
			}

			if val, ok := serviceAnnotations(service.ObjectMeta.Annotations).getHost(); ok {
				newSvc.Host = val
			}

			if val, ok := serviceAnnotations(service.ObjectMeta.Annotations).getAlgorithm(); ok {
				for _, current := range supportedAlgorithms {
					if val == current {
						newSvc.Algorithm = val
						break
					}
				}
			} else {
				newSvc.Algorithm = defaultAlgorithm
			}
			glog.Infof("Found service: %+v", newSvc)
		}
	}

	sort.Sort(serviceByName(tcpServices))
	sort.Sort(serviceByName(udpServices))

	return
}
