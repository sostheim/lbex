package nginx

import (
	"fmt"
	"os"
	"path"
	"text/template"

	"github.com/golang/glog"
)

const dhparamFilename = "dhparam.pem"

// HTTPNginxConfig describes an NGINX configuration primarily for Ingress Resource handling
type HTTPNginxConfig struct {
	Upstreams []Upstream
	Servers   []Server
}

// Upstream describes an NGINX upstream (context http)
// http://nginx.org/en/docs/http/ngx_http_upstream_module.html#upstream
type Upstream struct {
	Name            string
	UpstreamServers []UpstreamServer
}

// UpstreamServer describes a server in an NGINX upstream (context http::upstream)
// http://nginx.org/en/docs/http/ngx_http_upstream_module.html#server
type UpstreamServer struct {
	Address string
	Port    string
}

// Server describes an NGINX server
// http://nginx.org/en/docs/http/ngx_http_core_module.html
type Server struct {
	ServerSnippets        []string
	Name                  string
	ServerTokens          bool
	Locations             []Location
	SSL                   bool
	SSLCertificate        string
	SSLCertificateKey     string
	HTTP2                 bool
	RedirectToHTTPS       bool
	ProxyProtocol         bool
	HSTS                  bool
	HSTSMaxAge            int64
	HSTSIncludeSubdomains bool
	ProxyHideHeaders      []string
	ProxyPassHeaders      []string

	// http://nginx.org/en/docs/http/ngx_http_realip_module.html
	RealIPHeader    string
	SetRealIPFrom   []string
	RealIPRecursive bool
}

// Location describes an NGINX location
type Location struct {
	LocationSnippets     []string
	Path                 string
	Upstream             Upstream
	ProxyConnectTimeout  string
	ProxyReadTimeout     string
	ClientMaxBodySize    string
	Websocket            bool
	Rewrite              string
	SSL                  bool
	ProxyBuffering       bool
	ProxyBuffers         string
	ProxyBufferSize      string
	ProxyMaxTempFileSize string
}

// NewUpstreamWithDefaultServer creates an upstream with the default server.
// proxy_pass to an upstream with the default server returns 502.
// We use it for services that have no endpoints
func NewUpstreamWithDefaultServer(name string) Upstream {
	return Upstream{
		Name:            name,
		UpstreamServers: []UpstreamServer{UpstreamServer{Address: "127.0.0.1", Port: "8181"}},
	}
}

// DeleteHTTPConfiguration deletes the configuration file, which corresponds for the
// specified HTTP resource / service load balancer from NGINX conf directory
func (ngxc *NginxController) DeleteHTTPConfiguration(name string) {
	filename := ngxc.getHTTPConfigFileName(name)
	glog.V(3).Infof("deleting %v", filename)

	if ngxc.cfgType != LocalCfg {
		if err := os.Remove(filename); err != nil {
			glog.Warningf("Failed to delete %v: %v", filename, err)
		}
	}
}

// AddOrUpdateHTTPConfiguration creates or updates a configuration file with
// the specified configuration for the specified HTTP Configuration
func (ngxc *NginxController) AddOrUpdateHTTPConfiguration(name string, config HTTPNginxConfig) {
	glog.V(3).Infof("Updating NGINX configuration for HTTP Context: %v", name)
	filename := ngxc.getHTTPConfigFileName(name)
	ngxc.templateHTTP(config, filename)
}

// AddOrUpdateDHParam creates the servers dhparam.pem file
func (ngxc *NginxController) AddOrUpdateDHParam(dhparam string) (string, error) {
	fileName := ngxc.nginxCertsPath + "/" + dhparamFilename
	if ngxc.cfgType != LocalCfg {
		pem, err := os.Create(fileName)
		if err != nil {
			return fileName, fmt.Errorf("Couldn't create file %v: %v", fileName, err)
		}
		defer pem.Close()

		_, err = pem.WriteString(dhparam)
		if err != nil {
			return fileName, fmt.Errorf("Couldn't write to pem file %v: %v", fileName, err)
		}
	}
	return fileName, nil
}

// AddOrUpdateCertAndKey creates a .pem file wth the cert and the key with the
// specified name
func (ngxc *NginxController) AddOrUpdateCertAndKey(name string, cert string, key string) string {
	pemFileName := ngxc.nginxCertsPath + "/" + name + ".pem"

	if ngxc.cfgType != LocalCfg {
		pem, err := os.Create(pemFileName)
		if err != nil {
			glog.Fatalf("Couldn't create pem file %v: %v", pemFileName, err)
		}
		defer pem.Close()

		_, err = pem.WriteString(key)
		if err != nil {
			glog.Fatalf("Couldn't write to pem file %v: %v", pemFileName, err)
		}

		_, err = pem.WriteString("\n")
		if err != nil {
			glog.Fatalf("Couldn't write to pem file %v: %v", pemFileName, err)
		}

		_, err = pem.WriteString(cert)
		if err != nil {
			glog.Fatalf("Couldn't write to pem file %v: %v", pemFileName, err)
		}
	}

	return pemFileName
}

func (ngxc *NginxController) getHTTPConfigFileName(name string) string {
	return path.Join(ngxc.nginxConfdPath, name+".http.conf")
}

func (ngxc *NginxController) templateHTTP(config HTTPNginxConfig, filename string) {
	tmpl, err := template.New("http.tmpl").ParseFiles("http.tmpl")
	if err != nil {
		glog.Fatalf("failed to parse HTTP template file: %v", err)
	}

	if glog.V(3) {
		glog.Infof("writing NGINX HTTP configuration to %v", filename)
		tmpl.Execute(os.Stdout, config)
	}

	if ngxc.cfgType != LocalCfg {
		w, err := os.Create(filename)
		if err != nil {
			glog.Fatalf("failed to open %v: %v", filename, err)
		}
		defer w.Close()

		if err := tmpl.Execute(w, config); err != nil {
			glog.Fatalf("failed to write template %v", err)
		}
	}
}
