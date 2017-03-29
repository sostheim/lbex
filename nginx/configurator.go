package nginx

import (
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/golang/glog"
	"k8s.io/client-go/pkg/api/v1"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apis/extensions"
)

const emptyHost = ""

// Configurator transforms an Ingress or Service resource into NGINX Configuration
type Configurator struct {
	ngxc   *NginxController
	config *HTTPConfig
	lock   sync.Mutex
}

// NewConfigurator creates a new Configurator
func NewConfigurator(ngxc *NginxController, config *HTTPConfig) *Configurator {
	cfgtor := Configurator{
		ngxc:   ngxc,
		config: config,
	}
	return &cfgtor
}

func (cfgtor *Configurator) AddOrUpdateDHParam(content string) (string, error) {
	if cfgtor.ngxc.cfgType != IngressCfg {
		return "", errors.New("AddOrUpdateDHParam: I'm sorry Dave, I'm afraid I can't do that.")
	}
	return cfgtor.ngxc.AddOrUpdateDHParam(content)
}

// AddOrUpdateIngress adds or updates NGINX configuration for an Ingress resource
func (cfgtor *Configurator) AddOrUpdateIngress(name string, ingEx *IngressEx) error {
	if cfgtor.ngxc.cfgType != IngressCfg {
		return errors.New("AddOrUpdateIngress: I'm sorry Dave, I'm afraid I can't do that.")
	}

	cfgtor.lock.Lock()
	defer cfgtor.lock.Unlock()

	pems := cfgtor.updateCertificates(ingEx)
	nginxCfg := cfgtor.generateNginxIngressCfg(ingEx, pems)
	cfgtor.ngxc.AddOrUpdateIngress(name, nginxCfg)
	if err := cfgtor.ngxc.Reload(); err != nil {
		glog.Errorf("error on reload adding or updating ingress %q: %q", name, err)
	}
	return nil
}

// AddOrUpdateService adds or updates NGINX configuration for an Service object
func (cfgtor *Configurator) AddOrUpdateService(name string, svc *ServiceSpec) error {
	if cfgtor.ngxc.cfgType != ServiceCfg {
		return errors.New("AddOrUpdateService: I'm sorry Dave, I'm afraid I can't do that.")
	}

	cfgtor.lock.Lock()
	defer cfgtor.lock.Unlock()

	nginxCfg := cfgtor.generateNginxServiceCfg(svc)
	cfgtor.ngxc.AddOrUpdateService(name, nginxCfg)
	if err := cfgtor.ngxc.Reload(); err != nil {
		glog.Errorf("error on reload adding or updating service %q: %q", name, err)
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

func (cfgtor *Configurator) generateNginxIngressCfg(ingEx *IngressEx, pems map[string]string) IngressNginxConfig {
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

	return IngressNginxConfig{Upstreams: upstreamMapToSlice(upstreams), Servers: servers}
}

func (cfgtor *Configurator) generateNginxServiceCfg(svc *ServiceSpec) ServiceNginxConfig {
	svcCfg := cfgtor.createServiceConfig(svc)
	svcCfg.MainLogFormat = ""

	upstreams := make(map[string]StreamUpstream)

	name := getNameForStreamUpstream(svc.Service, emptyHost)
	upstream := cfgtor.createStreamUpstream(svc, name)
	upstreams[name] = upstream

	var servers []StreamServer

	for _, servicePort := range svc.Service.Spec.Ports {

		portName := servicePort.Name

		server := StreamServer{
			Listen: StreamListen{Address: portName},
		}
		servers = append(servers, server)
	}

	return ServiceNginxConfig{Upstreams: streamUpstreamMapToSlice(upstreams), Servers: servers}
}

func (cfgtor *Configurator) createIngressConfig(ingEx *IngressEx) HTTPConfig {
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

/* FIXME Next ! */
func (cfgtor *Configurator) createServiceConfig(svc *ServiceSpec) HTTPConfig {
	svcCfg := *cfgtor.config
	return svcCfg
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
		return "", "", fmt.Errorf("Invalid rewrite format: %s\n", service)
	}

	svcNameParts := strings.Split(parts[0], "=")
	if len(svcNameParts) != 2 {
		return "", "", fmt.Errorf("Invalid rewrite format: %s\n", svcNameParts)
	}

	rwPathParts := strings.Split(parts[1], "=")
	if len(rwPathParts) != 2 {
		return "", "", fmt.Errorf("Invalid rewrite format: %s\n", rwPathParts)
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

func createLocation(path string, upstream Upstream, cfg *HTTPConfig, websocket bool, rewrite string, ssl bool) Location {
	loc := Location{
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

	return loc
}

func (cfgtor *Configurator) createUpstream(ingEx *IngressEx, name string, backend *extensions.IngressBackend, namespace string) Upstream {
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

func (cfgtor *Configurator) createStreamUpstream(svc *ServiceSpec, name string) StreamUpstream {
	su := NewStreamUpstreamWithDefaultServer(name)

	endps, exists := svc.Endpoints[svc.Service.Namespace+"/"+svc.Service.Name]
	if exists {
		var suServers []StreamUpstreamServer
		for _, endp := range endps {
			suServers = append(suServers, StreamUpstreamServer{Address: endp})
		}
		if len(suServers) > 0 {
			su.UpstreamServers = suServers
		}
	}

	return su
}

func pathOrDefault(path string) string {
	if path == "" {
		return "/"
	}
	return path
}

func getNameForUpstream(ing *extensions.Ingress, host string, service string) string {
	return fmt.Sprintf("%v-%v-%v-%v", ing.Namespace, ing.Name, host, service)
}

func getNameForStreamUpstream(svc *v1.Service, host string) string {
	return fmt.Sprintf("%v-%v-%v", svc.Namespace, svc.Name, host)
}

func upstreamMapToSlice(upstreams map[string]Upstream) []Upstream {
	result := make([]Upstream, 0, len(upstreams))
	for _, ups := range upstreams {
		result = append(result, ups)
	}
	return result
}

func streamUpstreamMapToSlice(upstreams map[string]StreamUpstream) []StreamUpstream {
	result := make([]StreamUpstream, 0, len(upstreams))
	for _, ups := range upstreams {
		result = append(result, ups)
	}
	return result
}

// DeleteConfiguration deletes NGINX configuration for an Ingress Resource or Service LoadBalancer
func (cfgtor *Configurator) DeleteConfiguration(name string) {
	cfgtor.lock.Lock()
	defer cfgtor.lock.Unlock()

	cfgtor.ngxc.DeleteConfiguration(name)
	if err := cfgtor.ngxc.Reload(); err != nil {
		glog.Errorf("error on reload, removing configuration: %q: %q", name, err)
	}
}

// UpdateIngressEndpoints updates endpoints in NGINX configuration for an Ingress resource
func (cfgtor *Configurator) UpdateIngressEndpoints(name string, ingEx *IngressEx) error {
	if cfgtor.ngxc.cfgType != IngressCfg {
		return errors.New("UpdateIngressEndpoints: I'm sorry Dave, I'm afraid I can't do that.")
	}
	cfgtor.AddOrUpdateIngress(name, ingEx)
	return nil
}

// UpdateServiceEndpoints updates endpoints in NGINX configuration for a Service
func (cfgtor *Configurator) UpdateServiceEndpoints(name string, svc *ServiceSpec) error {
	if cfgtor.ngxc.cfgType != ServiceCfg {
		return errors.New("UpdateIngressEndpoints: I'm sorry Dave, I'm afraid I can't do that.")
	}
	cfgtor.AddOrUpdateService(name, svc)
	return nil
}

// UpdateMainConfigHTTPContext updates NGINX Configuration parameters
func (cfgtor *Configurator) UpdateMainConfigHTTPContext(config *HTTPConfig) error {
	if cfgtor.ngxc.cfgType != IngressCfg {
		return errors.New("UpdateMainConfig: I'm sorry Dave, I'm afraid I can't do that.")
	}
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
