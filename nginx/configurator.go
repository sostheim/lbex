package nginx

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/golang/glog"
	"github.com/sostheim/lbex/annotations"
	"k8s.io/client-go/pkg/api"
	v1 "k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/pkg/apis/extensions/v1beta1"
)

const emptyHost = ""
const udpProto = "udp"

// SingleDefaultPortName - provide a default name for a port that doesn't required one
const SingleDefaultPortName = "unnamed"

var (
	// map node names (key) to Node type
	nodes = make(map[string]Node)

	// map service key to nodes that populate the services upstream
	serviceUpstreamNodes = make(map[string][]Node)

	// map service key to the target that populate the services upstream
	serviceUpstreamTarget = make(map[string][]Target)

	// Why aren't these two maps combined in to a map of []inerface{} types
	// so we can just insert either Nodes or Targets against the same key?
	// See this discussion: https://github.com/golang/go/wiki/InterfaceSlice
)

// Configurator transforms an Ingress or Service resource into NGINX Configuration
type Configurator struct {
	ngxc   *NginxController
	config *HTTPContext
	lock   sync.Mutex
}

// NewConfigurator creates a new Configurator
func NewConfigurator(ngxc *NginxController) *Configurator {
	return &Configurator{
		ngxc:   ngxc,
		config: NewDefaultHTTPContext(),
	}
}

func serviceListByNodeAddress(address string) (list []string) {
	// TODO: should probably replace this nested loop search with a reverse map -> service keys
	for svc, upstreamNodes := range serviceUpstreamNodes {
		for _, node := range upstreamNodes {
			if node.InternalIP == address || node.ExternalIP == address {
				list = append(list, svc)
				break
			}
		}
	}
	return
}

func serviceListByNodeName(name string) (list []string) {
	// TODO: should probably replace this nested loop search with a reverse map -> service keys
	for svc, upstreamNodes := range serviceUpstreamNodes {
		for _, node := range upstreamNodes {
			if node.Name == name {
				list = append(list, svc)
				break
			}
		}
	}
	return
}

// AddOrUpdateNode - add, update (including removing) the node from the set of upstream candidates
func (cfgtor *Configurator) AddOrUpdateNode(node Node) []string {
	cfgtor.lock.Lock()
	defer cfgtor.lock.Unlock()

	services := []string{}
	elem, ok := nodes[node.Name]
	if !ok {
		glog.V(4).Infof("add new node: %v", node)
		nodes[node.Name] = node
	} else {
		if node.Active {
			glog.V(4).Infof("update existing active node: %v", node)
			nodes[node.Name] = node
			if elem.InternalIP != node.InternalIP {
				services = serviceListByNodeAddress(elem.InternalIP)
			}
			if elem.ExternalIP != node.ExternalIP {
				services = append(services, serviceListByNodeAddress(elem.ExternalIP)...)
			}
		} else {
			glog.V(4).Infof("update (delete) existing inactive node: %v", node)
			delete(nodes, node.Name)
			services = serviceListByNodeName(node.Name)
		}
	}
	return services
}

// DeleteNode - removes the node (if it exists) from the nodeIPAddresses slice
func (cfgtor *Configurator) DeleteNode(key string) []string {
	node, ok := nodes[key]
	if ok {
		node.Active = false
		return cfgtor.AddOrUpdateNode(node)
	}
	return nil
}

// AddOrUpdateDHParam adds the content string to parameters
func (cfgtor *Configurator) AddOrUpdateDHParam(content string) (string, error) {
	if cfgtor.ngxc.cfgType != HTTPCfg && cfgtor.ngxc.cfgType != StreamHTTPCfg {
		return "", errors.New("addOrUpdateDHParam: I'm sorry Dave, I'm afraid I can't do that")
	}
	return cfgtor.ngxc.AddOrUpdateDHParam(content)
}

// AddOrUpdateIngress adds or updates NGINX configuration for an Ingress resource
func (cfgtor *Configurator) AddOrUpdateIngress(name string, ingEx *IngressEx) error {
	if cfgtor.ngxc.cfgType != HTTPCfg && cfgtor.ngxc.cfgType != StreamHTTPCfg {
		return errors.New("addOrUpdateIngress: I'm sorry Dave, I'm afraid I can't do that")
	}

	cfgtor.lock.Lock()
	defer cfgtor.lock.Unlock()

	pems := cfgtor.updateCertificates(ingEx)
	nginxCfg := cfgtor.generateNginxIngressCfg(ingEx, pems)
	cfgtor.ngxc.AddOrUpdateHTTPConfiguration(name, nginxCfg)
	if err := cfgtor.ngxc.Reload(); err != nil {
		glog.Errorf("error on reload adding or updating ingress %q: %q", name, err)
	}
	return nil
}

// AddOrUpdateService adds or updates NGINX configuration for an Service object
func (cfgtor *Configurator) AddOrUpdateService(svc *ServiceSpec) error {
	if cfgtor.ngxc.cfgType != StreamCfg && cfgtor.ngxc.cfgType != StreamHTTPCfg {
		return errors.New("addOrUpdateService: I'm sorry Dave, I'm afraid I can't do that")
	}

	cfgtor.lock.Lock()
	defer cfgtor.lock.Unlock()

	nginxCfg := cfgtor.generateStreamNginxConfig(svc)
	cfgtor.ngxc.AddOrUpdateStream(svc.ConfigName, nginxCfg)
	if err := cfgtor.ngxc.Reload(); err != nil {
		glog.Errorf("error on reload adding or updating service %q: %q", svc.ConfigName, err)
	}
	return nil
}

func (cfgtor *Configurator) updateCertificates(ingEx *IngressEx) map[string]string {
	pems := make(map[string]string)

	for _, tls := range ingEx.Ingress.Spec.TLS {
		secretName := tls.SecretName
		secret, exist := ingEx.Secrets[secretName]
		if !exist {
			continue
		}
		cert, ok := secret.Data[api.TLSCertKey]
		if !ok {
			glog.Warningf("Secret %v has no private key", secretName)
			continue
		}
		key, ok := secret.Data[api.TLSPrivateKeyKey]
		if !ok {
			glog.Warningf("Secret %v has no cert", secretName)
			continue
		}

		name := ingEx.Ingress.Namespace + "-" + secretName
		pemFileName := cfgtor.ngxc.AddOrUpdateCertAndKey(name, string(cert), string(key))

		for _, host := range tls.Hosts {
			pems[host] = pemFileName
		}
		if len(tls.Hosts) == 0 {
			pems[emptyHost] = pemFileName
		}
	}

	return pems
}

func (cfgtor *Configurator) generateNginxIngressCfg(ingEx *IngressEx, pems map[string]string) HTTPNginxConfig {
	ingCfg := cfgtor.createIngressConfig(ingEx)

	upstreams := make(map[string]Upstream)

	wsServices := getWebsocketServices(ingEx)
	rewrites := getRewrites(ingEx)
	sslServices := getSSLServices(ingEx)

	if ingEx.Ingress.Spec.Backend != nil {
		name := getNameForUpstream(ingEx.Ingress, emptyHost, ingEx.Ingress.Spec.Backend.ServiceName)
		upstream := cfgtor.createUpstream(ingEx, name, ingEx.Ingress.Spec.Backend, ingEx.Ingress.Namespace)
		upstreams[name] = upstream
	}

	var servers []Server

	for _, rule := range ingEx.Ingress.Spec.Rules {
		if rule.IngressRuleValue.HTTP == nil {
			continue
		}

		serverName := rule.Host

		if rule.Host == emptyHost {
			glog.Warningf("Host field of ingress rule in %v/%v is empty", ingEx.Ingress.Namespace, ingEx.Ingress.Name)
		}

		server := Server{
			Name:                  serverName,
			ServerTokens:          ingCfg.ServerTokens,
			HTTP2:                 ingCfg.HTTP2,
			RedirectToHTTPS:       ingCfg.RedirectToHTTPS,
			ProxyProtocol:         ingCfg.ProxyProtocol,
			HSTS:                  ingCfg.HSTS,
			HSTSMaxAge:            ingCfg.HSTSMaxAge,
			HSTSIncludeSubdomains: ingCfg.HSTSIncludeSubdomains,
			RealIPHeader:          ingCfg.RealIPHeader,
			SetRealIPFrom:         ingCfg.SetRealIPFrom,
			RealIPRecursive:       ingCfg.RealIPRecursive,
			ProxyHideHeaders:      ingCfg.ProxyHideHeaders,
			ProxyPassHeaders:      ingCfg.ProxyPassHeaders,
			ServerSnippets:        ingCfg.ServerSnippets,
		}

		if pemFile, ok := pems[serverName]; ok {
			server.SSL = true
			server.SSLCertificate = pemFile
			server.SSLCertificateKey = pemFile
		}

		var locations []Location
		rootLocation := false

		for _, path := range rule.HTTP.Paths {
			upsName := getNameForUpstream(ingEx.Ingress, rule.Host, path.Backend.ServiceName)

			if _, exists := upstreams[upsName]; !exists {
				upstream := cfgtor.createUpstream(ingEx, upsName, &path.Backend, ingEx.Ingress.Namespace)
				upstreams[upsName] = upstream
			}

			loc := createLocation(pathOrDefault(path.Path), upstreams[upsName], &ingCfg, wsServices[path.Backend.ServiceName], rewrites[path.Backend.ServiceName], sslServices[path.Backend.ServiceName])
			locations = append(locations, loc)

			if loc.Path == "/" {
				rootLocation = true
			}
		}

		if rootLocation == false && ingEx.Ingress.Spec.Backend != nil {
			upsName := getNameForUpstream(ingEx.Ingress, emptyHost, ingEx.Ingress.Spec.Backend.ServiceName)
			loc := createLocation(pathOrDefault("/"), upstreams[upsName], &ingCfg, wsServices[ingEx.Ingress.Spec.Backend.ServiceName], rewrites[ingEx.Ingress.Spec.Backend.ServiceName], sslServices[ingEx.Ingress.Spec.Backend.ServiceName])
			locations = append(locations, loc)
		}

		server.Locations = locations
		servers = append(servers, server)
	}

	if len(ingEx.Ingress.Spec.Rules) == 0 && ingEx.Ingress.Spec.Backend != nil {
		server := Server{
			Name:                  emptyHost,
			ServerTokens:          ingCfg.ServerTokens,
			HTTP2:                 ingCfg.HTTP2,
			RedirectToHTTPS:       ingCfg.RedirectToHTTPS,
			ProxyProtocol:         ingCfg.ProxyProtocol,
			HSTS:                  ingCfg.HSTS,
			HSTSMaxAge:            ingCfg.HSTSMaxAge,
			HSTSIncludeSubdomains: ingCfg.HSTSIncludeSubdomains,
			RealIPHeader:          ingCfg.RealIPHeader,
			SetRealIPFrom:         ingCfg.SetRealIPFrom,
			RealIPRecursive:       ingCfg.RealIPRecursive,
			ProxyHideHeaders:      ingCfg.ProxyHideHeaders,
			ProxyPassHeaders:      ingCfg.ProxyPassHeaders,
			ServerSnippets:        ingCfg.ServerSnippets,
		}

		if pemFile, ok := pems[emptyHost]; ok {
			server.SSL = true
			server.SSLCertificate = pemFile
			server.SSLCertificateKey = pemFile
		}

		var locations []Location

		upsName := getNameForUpstream(ingEx.Ingress, emptyHost, ingEx.Ingress.Spec.Backend.ServiceName)

		loc := createLocation(pathOrDefault("/"), upstreams[upsName], &ingCfg, wsServices[ingEx.Ingress.Spec.Backend.ServiceName], rewrites[ingEx.Ingress.Spec.Backend.ServiceName], sslServices[ingEx.Ingress.Spec.Backend.ServiceName])
		locations = append(locations, loc)

		server.Locations = locations
		servers = append(servers, server)
	}

	return HTTPNginxConfig{Upstreams: upstreamMapToSlice(upstreams), Servers: servers}
}

func (cfgtor *Configurator) generateStreamNginxConfig(svc *ServiceSpec) (svcConfig StreamNginxConfig) {
	glog.V(4).Infof("create StreamNginxConfig for svc: %s, spec: %s", svc.Key, svc)

	if val, ok := annotations.GetOptionalStringAnnotation(annotations.LBEXResolverKey, svc.Service); ok {
		svcConfig.Resolver = val
	}

	upstreams := make(map[string]*StreamUpstream)

	for _, target := range svc.Topology {
		var upstream StreamUpstream
		switch svc.UpstreamType {
		case HostNode:
			upstream = cfgtor.createNodesStreamUpstream(svc, target)
		case Pod:
			upstream = cfgtor.createPodStreamUpstream(svc, target)
		case ClusterIP:
			upstream = cfgtor.createClusterStreamUpstream(svc, target)
		default:
			glog.Warningf("hit a switch case DEFAULT <---> %v", svc.UpstreamType)
		}

		elem, exists := upstreams[upstream.Name]
		if !exists {
			upstreams[upstream.Name] = &upstream
			// Since RR is the default and diretives only over-ride the default,
			// you *can't* set "roundrobin", or the configuration will be rejected.
			if svc.Algorithm != RoundRobin {
				upstream.Algorithm = svc.Algorithm
			}
			if upstream.Algorithm == LowestLatency {
				val, _ := annotations.GetOptionalStringAnnotation(annotations.LBEXMethodKey, svc.Service)
				upstream.LeastTimeMethod = ValidateMethod(val)
			}

			portAnnotation := annotations.LBEXPortAnnotationBase + target.PortName
			listenPort, err := annotations.GetIntAnnotation(portAnnotation, svc.Service)
			if err != nil {
				if annotations.IsMissingAnnotations(err) {
					glog.Warningf("Annotation %s is not present", portAnnotation)
				} else {
					glog.V(2).Infof("unexpected error processing annotation, err: %v", err)
				}

				continue
			}

			passThrough, _ := annotations.GetOptionalBoolAnnotation(annotations.LBEXIpPassthrough, svc.Service)

			server := StreamServer{
				Listen: StreamListen{
					Port: strconv.Itoa(listenPort),
					UDP:  strings.EqualFold(target.Protocol, udpProto),
				},
				ProxyProtocol:    false,
				ProxyPassthrough: passThrough,
				ProxyPassAddress: upstream.Name,
			}
			svcConfig.Servers = append(svcConfig.Servers, server)
		} else {
			elem.UpstreamServers = append(elem.UpstreamServers, upstream.UpstreamServers...)
			upstreams[upstream.Name] = elem
		}
	}

	for _, up := range upstreams {
		svcConfig.Upstreams = append(svcConfig.Upstreams, *up)
	}

	glog.V(4).Infof("created StreamNginxConfig: %s", svcConfig)

	return
}

func (cfgtor *Configurator) createIngressConfig(ingEx *IngressEx) HTTPContext {
	ingCfg := *cfgtor.config
	if serverTokens, exists, err := GetMapKeyAsBool(ingEx.Ingress.Annotations, "nginx.org/server-tokens", ingEx.Ingress); exists {
		if err != nil {
			glog.Error(err)
		} else {
			ingCfg.ServerTokens = serverTokens
		}
	}

	if serverSnippets, exists, err := GetMapKeyAsStringSlice(ingEx.Ingress.Annotations, "nginx.org/server-snippets", ingEx.Ingress, "\n"); exists {
		if err != nil {
			glog.Error(err)
		} else {
			ingCfg.ServerSnippets = serverSnippets
		}
	}
	if locationSnippets, exists, err := GetMapKeyAsStringSlice(ingEx.Ingress.Annotations, "nginx.org/location-snippets", ingEx.Ingress, "\n"); exists {
		if err != nil {
			glog.Error(err)
		} else {
			ingCfg.LocationSnippets = locationSnippets
		}
	}

	if proxyConnectTimeout, exists := ingEx.Ingress.Annotations["nginx.org/proxy-connect-timeout"]; exists {
		ingCfg.ProxyConnectTimeout = proxyConnectTimeout
	}
	if proxyReadTimeout, exists := ingEx.Ingress.Annotations["nginx.org/proxy-read-timeout"]; exists {
		ingCfg.ProxyReadTimeout = proxyReadTimeout
	}
	if proxyHideHeaders, exists, err := GetMapKeyAsStringSlice(ingEx.Ingress.Annotations, "nginx.org/proxy-hide-headers", ingEx.Ingress, ","); exists {
		if err != nil {
			glog.Error(err)
		} else {
			ingCfg.ProxyHideHeaders = proxyHideHeaders
		}
	}
	if proxyPassHeaders, exists, err := GetMapKeyAsStringSlice(ingEx.Ingress.Annotations, "nginx.org/proxy-pass-headers", ingEx.Ingress, ","); exists {
		if err != nil {
			glog.Error(err)
		} else {
			ingCfg.ProxyPassHeaders = proxyPassHeaders
		}
	}
	if clientMaxBodySize, exists := ingEx.Ingress.Annotations["nginx.org/client-max-body-size"]; exists {
		ingCfg.ClientMaxBodySize = clientMaxBodySize
	}
	if HTTP2, exists, err := GetMapKeyAsBool(ingEx.Ingress.Annotations, "nginx.org/http2", ingEx.Ingress); exists {
		if err != nil {
			glog.Error(err)
		} else {
			ingCfg.HTTP2 = HTTP2
		}
	}
	if redirectToHTTPS, exists, err := GetMapKeyAsBool(ingEx.Ingress.Annotations, "nginx.org/redirect-to-https", ingEx.Ingress); exists {
		if err != nil {
			glog.Error(err)
		} else {
			ingCfg.RedirectToHTTPS = redirectToHTTPS
		}
	}
	if proxyBuffering, exists, err := GetMapKeyAsBool(ingEx.Ingress.Annotations, "nginx.org/proxy-buffering", ingEx.Ingress); exists {
		if err != nil {
			glog.Error(err)
		} else {
			ingCfg.ProxyBuffering = proxyBuffering
		}
	}

	if hsts, exists, err := GetMapKeyAsBool(ingEx.Ingress.Annotations, "nginx.org/hsts", ingEx.Ingress); exists {
		if err != nil {
			glog.Error(err)
		} else {
			parsingErrors := false

			hstsMaxAge, existsMA, err := GetMapKeyAsInt(ingEx.Ingress.Annotations, "nginx.org/hsts-max-age", ingEx.Ingress)
			if existsMA && err != nil {
				glog.Error(err)
				parsingErrors = true
			}
			hstsIncludeSubdomains, existsIS, err := GetMapKeyAsBool(ingEx.Ingress.Annotations, "nginx.org/hsts-include-subdomains", ingEx.Ingress)
			if existsIS && err != nil {
				glog.Error(err)
				parsingErrors = true
			}

			if parsingErrors {
				glog.Errorf("Ingress %s/%s: There are configuration issues with hsts annotations, skipping annotions for all hsts settings", ingEx.Ingress.GetNamespace(), ingEx.Ingress.GetName())
			} else {
				ingCfg.HSTS = hsts
				if existsMA {
					ingCfg.HSTSMaxAge = hstsMaxAge
				}
				if existsIS {
					ingCfg.HSTSIncludeSubdomains = hstsIncludeSubdomains
				}
			}
		}
	}

	if proxyBuffers, exists := ingEx.Ingress.Annotations["nginx.org/proxy-buffers"]; exists {
		ingCfg.ProxyBuffers = proxyBuffers
	}
	if proxyBufferSize, exists := ingEx.Ingress.Annotations["nginx.org/proxy-buffer-size"]; exists {
		ingCfg.ProxyBufferSize = proxyBufferSize
	}
	if proxyMaxTempFileSize, exists := ingEx.Ingress.Annotations["nginx.org/proxy-max-temp-file-size"]; exists {
		ingCfg.ProxyMaxTempFileSize = proxyMaxTempFileSize
	}
	return ingCfg
}

func getWebsocketServices(ingEx *IngressEx) map[string]bool {
	wsServices := make(map[string]bool)

	if services, exists := ingEx.Ingress.Annotations["nginx.org/websocket-services"]; exists {
		for _, svc := range strings.Split(services, ",") {
			wsServices[svc] = true
		}
	}

	return wsServices
}

func getRewrites(ingEx *IngressEx) map[string]string {
	rewrites := make(map[string]string)

	if services, exists := ingEx.Ingress.Annotations["nginx.org/rewrites"]; exists {
		for _, svc := range strings.Split(services, ";") {
			if serviceName, rewrite, err := parseRewrites(svc); err != nil {
				glog.Errorf("In %v nginx.org/rewrites contains invalid declaration: %v, ignoring", ingEx.Ingress.Name, err)
			} else {
				rewrites[serviceName] = rewrite
			}
		}
	}

	return rewrites
}

func parseRewrites(service string) (serviceName string, rewrite string, err error) {
	parts := strings.SplitN(service, " ", 2)

	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid rewrite format: %s", service)
	}

	svcNameParts := strings.Split(parts[0], "=")
	if len(svcNameParts) != 2 {
		return "", "", fmt.Errorf("invalid rewrite format: %s", svcNameParts)
	}

	rwPathParts := strings.Split(parts[1], "=")
	if len(rwPathParts) != 2 {
		return "", "", fmt.Errorf("invalid rewrite format: %s", rwPathParts)
	}

	return svcNameParts[1], rwPathParts[1], nil
}

func getSSLServices(ingEx *IngressEx) map[string]bool {
	sslServices := make(map[string]bool)

	if services, exists := ingEx.Ingress.Annotations["nginx.org/ssl-services"]; exists {
		for _, svc := range strings.Split(services, ",") {
			sslServices[svc] = true
		}
	}

	return sslServices
}

func createLocation(path string, upstream Upstream, cfg *HTTPContext, websocket bool, rewrite string, ssl bool) Location {
	return Location{
		Path:                 path,
		Upstream:             upstream,
		ProxyConnectTimeout:  cfg.ProxyConnectTimeout,
		ProxyReadTimeout:     cfg.ProxyReadTimeout,
		ClientMaxBodySize:    cfg.ClientMaxBodySize,
		Websocket:            websocket,
		Rewrite:              rewrite,
		SSL:                  ssl,
		ProxyBuffering:       cfg.ProxyBuffering,
		ProxyBuffers:         cfg.ProxyBuffers,
		ProxyBufferSize:      cfg.ProxyBufferSize,
		ProxyMaxTempFileSize: cfg.ProxyMaxTempFileSize,
		LocationSnippets:     cfg.LocationSnippets,
	}
}

func (cfgtor *Configurator) createUpstream(ingEx *IngressEx, name string, backend *v1beta1.IngressBackend, namespace string) Upstream {
	ups := NewUpstreamWithDefaultServer(name)

	endps, exists := ingEx.Endpoints[backend.ServiceName+backend.ServicePort.String()]
	if exists {
		var upsServers []UpstreamServer
		for _, endp := range endps {
			addressport := strings.Split(endp, ":")
			upsServers = append(upsServers, UpstreamServer{addressport[0], addressport[1]})
		}
		if len(upsServers) > 0 {
			ups.UpstreamServers = upsServers
		}
	}
	return ups
}

func (cfgtor *Configurator) createClusterStreamUpstream(spec *ServiceSpec, target Target) StreamUpstream {
	serviceUpstreamTarget[spec.Key] = append(serviceUpstreamTarget[spec.Key], target)
	return StreamUpstream{
		Name: getNameForStreamUpstream(spec.Service, target.PortName),
		UpstreamServers: []StreamUpstreamServer{
			{Address: spec.ClusterIP + ":" + strconv.Itoa(target.ServicePort)}},
	}
}

func (cfgtor *Configurator) createPodStreamUpstream(spec *ServiceSpec, target Target) StreamUpstream {
	serviceUpstreamTarget[spec.Key] = append(serviceUpstreamTarget[spec.Key], target)
	return StreamUpstream{
		Name: getNameForStreamUpstream(spec.Service, target.PortName),
		UpstreamServers: []StreamUpstreamServer{
			{Address: target.PodIP + ":" + strconv.Itoa(target.PodPort)}},
	}
}

func (cfgtor *Configurator) createNodesStreamUpstream(spec *ServiceSpec, target Target) StreamUpstream {
	val, _ := annotations.GetOptionalStringAnnotation(annotations.LBEXNodeSet, spec.Service)
	set := ValidateNodeSet(val)

	val, _ = annotations.GetOptionalStringAnnotation(annotations.LBEXNodeAddressType, spec.Service)
	addressType := ValidateNodeAddressType(val)

	su := StreamUpstream{
		Name: getNameForStreamUpstream(spec.Service, target.PortName),
	}
	glog.V(4).Infof("node set: %s, address type: %s, stream name: %s", set, addressType, su.Name)

	switch set {
	case Host:
		node, ok := nodes[target.NodeName]
		if !ok {
			glog.Warningf("no nodes map entry found for: %s", target.NodeName)
			break
		}
		su.UpstreamServers = append(su.UpstreamServers,
			StreamUpstreamServer{Address: formatAddress(addressType, &node, &target)})
		serviceUpstreamNodes[spec.Key] = []Node{node}

	case All:
		upstreamNodes := []Node{}
		for _, node := range nodes {
			su.UpstreamServers = append(su.UpstreamServers,
				StreamUpstreamServer{Address: formatAddress(addressType, &node, &target)})
			upstreamNodes = append(upstreamNodes, node)
		}
		serviceUpstreamNodes[spec.Key] = upstreamNodes

	default:
		glog.Warningf("hit a switch case DEFAULT <---> %s", set)
	}
	return su
}

func formatAddress(addrType string, node *Node, target *Target) string {
	var address string
	if addrType == Internal {
		address = node.InternalIP + ":" + strconv.Itoa(target.NodePort)
	} else {
		address = node.ExternalIP + ":" + strconv.Itoa(target.NodePort)
	}
	glog.V(4).Infof("formatted address: %s", address)
	return address
}

func pathOrDefault(path string) string {
	if path == "" {
		return "/"
	}
	return path
}

func getNameForUpstream(ing *v1beta1.Ingress, host string, service string) string {
	return fmt.Sprintf("%v-%v-%v-%v", ing.Namespace, ing.Name, host, service)
}

func getNameForStreamUpstream(svc *v1.Service, portName string) string {
	if portName == "" {
		// Port name can only be blank/omitted when there is a single port
		// defined for a service.  Any service with > 1 ports must provide
		// names for all ports that compose the services endpoints.
		portName = SingleDefaultPortName
	}
	return fmt.Sprintf("%v-%v-%v", svc.Namespace, svc.Name, portName)
}

func upstreamMapToSlice(upstreams map[string]Upstream) []Upstream {
	result := make([]Upstream, 0, len(upstreams))
	for _, ups := range upstreams {
		result = append(result, ups)
	}
	return result
}

// DeleteConfiguration deletes NGINX configuration for an Ingress Resource or Service LoadBalancer
func (cfgtor *Configurator) DeleteConfiguration(name string, cfgType Configuration) {
	cfgtor.lock.Lock()
	defer cfgtor.lock.Unlock()

	switch cfgType {
	case StreamCfg:
		cfgtor.ngxc.DeleteStreamConfiguration(name)
	case HTTPCfg:
		cfgtor.ngxc.DeleteHTTPConfiguration(name)
	case StreamHTTPCfg:
		cfgtor.ngxc.DeleteStreamConfiguration(name)
		cfgtor.ngxc.DeleteHTTPConfiguration(name)
	default:
		glog.Warningf("hit a switch case DEFAULT <---> %v", cfgType)
	}
	delete(serviceUpstreamNodes, name)
	delete(serviceUpstreamTarget, name)
	if err := cfgtor.ngxc.Reload(); err != nil {
		glog.Errorf("error on reload, removing configuration: %q: %q", name, err)
	}
}

// UpdateIngressEndpoints updates endpoints in NGINX configuration for an Ingress resource
func (cfgtor *Configurator) UpdateIngressEndpoints(name string, ingEx *IngressEx) error {
	if cfgtor.ngxc.cfgType != HTTPCfg && cfgtor.ngxc.cfgType != StreamHTTPCfg {
		return errors.New("updateIngressEndpoints: I'm sorry Dave, I'm afraid I can't do that")
	}
	cfgtor.AddOrUpdateIngress(name, ingEx)
	return nil
}

// UpdateServiceEndpoints updates endpoints in NGINX configuration for a Service
func (cfgtor *Configurator) UpdateServiceEndpoints(svc *ServiceSpec) error {
	if cfgtor.ngxc.cfgType != StreamCfg && cfgtor.ngxc.cfgType != StreamHTTPCfg {
		return errors.New("updateServiceEndpoints: I'm sorry Dave, I'm afraid I can't do that")
	}
	cfgtor.AddOrUpdateService(svc)
	return nil
}

// UpdateMainConfigHTTPContext updates NGINX Configuration parameters
func (cfgtor *Configurator) UpdateMainConfigHTTPContext(config *HTTPContext) error {
	cfgtor.lock.Lock()
	defer cfgtor.lock.Unlock()

	cfgtor.config = config
	cfgtor.ngxc.mainCfg.HTTPContext = NginxMainHTTPConfig{
		HTTPSnippets:              config.MainHTTPSnippets,
		ServerNamesHashBucketSize: config.MainServerNamesHashBucketSize,
		ServerNamesHashMaxSize:    config.MainServerNamesHashMaxSize,
		LogFormat:                 config.MainLogFormat,
		SSLProtocols:              config.MainServerSSLProtocols,
		SSLCiphers:                config.MainServerSSLCiphers,
		SSLDHParam:                config.MainServerSSLDHParam,
		SSLPreferServerCiphers:    config.MainServerSSLPreferServerCiphers,
	}
	cfgtor.ngxc.UpdateMainConfigFile()
	return nil
}
