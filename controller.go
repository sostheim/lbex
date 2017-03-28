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
	"runtime"
	"sort"
	"time"

	"github.com/golang/glog"
	"github.com/sostheim/lbex/annotations"
	"github.com/sostheim/lbex/nginx"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/pkg/util/intstr"
	"k8s.io/client-go/tools/cache"
)

var (
	resyncPeriod        = 30 * time.Second
	supportedAlgorithms = []string{
		"roundrobin", // *set as default below* direct traffic sequentially to the servers.
		"leastconn",  // selects the server with the smaller number of current active connections.
		"leasttime",  // selects the server with the lowest average latency and the least number of active connections.
	}
	defaultAlgorithm = string("roundrobin")
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

	endpointsLWC   *lwController
	endpointStore  cache.Store
	endpointsQueue *TaskQueue

	servciesLWC   *lwController
	servicesStore cache.Store
	servicesQueue *TaskQueue

	// The service to provide load balancing for, or "all" if empty
	service string

	stopCh chan struct{}

	cnf *nginx.Configurator
}

func newLbExController(client *dynamic.Client, clientset *kubernetes.Clientset, service *string) *lbExController {
	// local testing -> no actual NGINX instance
	local := ("darwin" == runtime.GOOS)

	glog.V(3).Infof("newLbExController: is local: %t", local)

	// Create and start the NGINX LoadBalancer
	ngxc, _ := nginx.NewNginxController(nginx.ServiceCfg, "/etc/nginx/", local, false)
	ngxc.Start()

	config := nginx.NewDefaultConfig()
	cnftor := nginx.NewConfigurator(ngxc, config)

	glog.V(3).Infof("newLbExController: NGINX server started w/ default configuration")

	// create external loadbalancer controller struct
	lbexc := lbExController{
		client:    client,
		clientset: clientset,
		stopCh:    make(chan struct{}),
		service:   *service,
		cnf:       cnftor,
	}
	lbexc.servicesQueue = NewTaskQueue(lbexc.syncServices)
	lbexc.servciesLWC = newServicesListWatchControllerForClientset(&lbexc)
	lbexc.endpointsQueue = NewTaskQueue(lbexc.syncEndpoints)
	lbexc.endpointsLWC = newEndpointsListWatchControllerForClientset(&lbexc)

	return &lbexc
}

func (lbex *lbExController) run() {
	// run the controller and queue goroutines
	go lbex.servciesLWC.controller.Run(lbex.stopCh)
	go lbex.endpointsLWC.controller.Run(lbex.stopCh)
	go lbex.servicesQueue.Run(time.Second, lbex.stopCh)
	go lbex.endpointsQueue.Run(time.Second, lbex.stopCh)
}

func (lbex *lbExController) syncServices(obj interface{}) error {

	if lbex.servicesQueue.IsShuttingDown() {
		return nil
	}

	key, ok := obj.(string)
	if !ok {
		return errors.New("syncServices: type assertion faild for key string")
	}

	storeObj, exists, err := lbex.servicesStore.GetByKey(key)
	if err != nil {
		glog.V(2).Infof("syncServices: deleting service: %v\n", key)
		lbex.cnf.DeleteConfiguration(key)
	} else if exists {
		glog.V(3).Infof("syncServices: updating services for key: %s", key)
		glog.V(4).Infof("syncServices: updating services object %v", storeObj)
		udpSvc, tcpSvc := lbex.getServices()
		if len(udpSvc) == 0 && len(tcpSvc) == 0 {
			glog.V(3).Info("syncServices: no services currently match criteria")
		} else {
			glog.V(3).Infof("syncServices: triggering configuration event for\nTCP Services: %v\nUDP Services: %v", tcpSvc, udpSvc)
		}
	} else {
		glog.V(3).Infof("syncServices: unable to find cached service object for key value: %s", key)
	}
	return nil
}

func (lbex *lbExController) syncEndpoints(obj interface{}) error {

	if lbex.endpointsQueue.IsShuttingDown() {
		return nil
	}

	key, ok := obj.(string)
	if !ok {
		return errors.New("syncEndpoints: invalid conversion from object any to key string")
	}

	storeObj, exists, err := lbex.endpointStore.GetByKey(key)
	if err != nil {
		return err
	} else if exists {
		glog.V(3).Infof("syncEndpoints: updating endpoints for key %s", key)
		glog.V(4).Infof("syncEndpoints: updating endpoint object %v", storeObj)
		udpSvc, tcpSvc := lbex.getServices()
		if len(udpSvc) == 0 && len(tcpSvc) == 0 {
			glog.V(3).Info("syncEndpoints: no services currently match criteria")
		} else {
			glog.V(3).Infof("syncEndpoints: triggering configuration event for:\nTCP Services: %v\nUDP Services: %v", tcpSvc, udpSvc)
		}
	} else {
		glog.V(3).Infof("syncEndpoints: unable to find cachd endpoint object for key value: %s", key)
	}
	return nil
}

// getServiceEndpoints returns the endpoints for the specified service name / namesapce.
func (lbex *lbExController) getServiceEndpoints(service *v1.Service) (endpoints v1.Endpoints, err error) {
	for _, ep := range lbex.endpointStore.List() {
		endpoints = *ep.(*v1.Endpoints)
		if service.Name == endpoints.Name && service.Namespace == endpoints.Namespace {
			return endpoints, nil
		}
	}
	err = fmt.Errorf("could not find endpoints for service: %v", service.Name)
	return
}

// getEndpoints returns a list of <endpoint ip>:<port> for a given service/target port combination.
func (lbex *lbExController) getEndpoints(service *v1.Service, servicePort *v1.ServicePort) (endpoints []string) {
	svcEndpoints, err := lbex.getServiceEndpoints(service)
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
				servicePortInt, err := GetServicePortTargetPortInt(servicePort)
				if err != nil {
					continue
				}
				if epPort.Port == int32(servicePortInt) {
					targetPort = servicePortInt
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
	objects := lbex.servicesStore.List()
	for _, obj := range objects {
		err := ValidateServiceObjectType(obj)
		if err != nil {
			glog.V(3).Infof("getServices: ValidateServiceObjectType(): err: %v", err)
			continue
		}

		serviceName, _ := GetServiceName(obj)
		if lbex.service != "" && lbex.service != serviceName {
			glog.V(3).Infof("getServices: ignoring non-matching service name: %s", serviceName)
			continue
		}

		if ServiceTypeLoadBalancer(obj) {
			glog.V(3).Infof("getServices: service: %s, has 'Type: LoadBalancer' - skipping", serviceName)
			continue
		}
		// Only services with the appropriate annotations are candidates
		if !annotations.IsValid(obj) {
			glog.V(3).Infof("getServices: service: %s, ignoring missing or non-matching loadbalancer.class", serviceName)
			continue
		}

		service, ok := obj.(*v1.Service)
		if !ok {
			glog.Warningf("getServices: service: %s, only V1 objects currently supported!", serviceName)
			continue
		}

		ep := []string{}
		for _, servicePort := range service.Spec.Ports {

			ep = lbex.getEndpoints(service, &servicePort)
			if len(ep) == 0 {
				glog.V(3).Infof("getServices: no endpoints found for service %s, port %+d", service.Name, servicePort)
				continue
			}
			backendPort, _ := GetServicePortTargetPortInt(&servicePort)
			newSvc := Service{
				Name:        GetServiceNameForLBRule(serviceName, int(servicePort.Port)),
				Ep:          ep,
				BackendPort: backendPort,
			}

			if val, ok := annotations.GetHost(service); ok {
				newSvc.Host = val
			}

			if val, ok := annotations.GetAlgorithm(service); ok {
				for _, current := range supportedAlgorithms {
					if val == current {
						newSvc.Algorithm = val
						break
					}
				}
			} else {
				newSvc.Algorithm = defaultAlgorithm
			}
			newSvc.FrontendPort = int(servicePort.Port)

			if servicePort.Protocol == v1.ProtocolUDP {
				udpServices = append(udpServices, newSvc)
			} else {
				tcpServices = append(tcpServices, newSvc)
			}

			glog.V(3).Infof("getServices: lbex supported service: %+v", newSvc)
		}

	}

	sort.Sort(serviceByName(tcpServices))
	sort.Sort(serviceByName(udpServices))

	return
}
