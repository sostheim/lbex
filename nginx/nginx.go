package nginx

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path"
	"reflect"
	"text/template"

	"github.com/golang/glog"
)

const dhparamFilename = "dhparam.pem"

type Configuration uint8

const (
	IngressCfg = Configuration(iota)
	ServiceCfg
)

// NginxController Updates NGINX configuration, starts and reloads NGINX
type NginxController struct {
	nginxConfdPath string
	nginxCertsPath string
	local          bool
	cfgType        Configuration
	mainCfg        *NginxMainConfig
}

// IngressNginxConfig describes an NGINX configuration Ingress Resource handling
type IngressNginxConfig struct {
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

// NginxMainConfig describe the main NGINX configuration file
type NginxMainConfig struct {
	// Context: main directives
	Daemon         bool
	ErrorLogFile   string
	ErrorLogLevel  string
	Environment    map[string]string
	LockFile       string
	PidFile        string
	User           string
	Group          string
	WorkerPriority string
	// TODO: This needs to be a ConfigMap entry or CLI flag so that we can make
	//       it a function of the number of CPUs/vCPUs, and configure the POD
	//       resource limits propotionally for the scheduler.  For now this
	//       *should probably not* be set to 'auto'
	WorkerProcesses  string
	WorkingDirectory string

	EventContext NginxMainEventConfig

	DefaultHTTPServer bool
	HTTPContext       NginxMainHTTPConfig
}

// NginxMainEventConfig describe the main NGINX configuration file's 'events' context
type NginxMainEventConfig struct {
	// Context: events directives
	AcceptMutex       bool
	AcceptMutexDelay  string
	MultiAccept       bool
	WorkerConnections string
}

// NginxMainHTTPConfig describe the main NGINX configuration file's 'http' context
type NginxMainHTTPConfig struct {
	ServerNamesHashBucketSize string
	ServerNamesHashMaxSize    string
	LogFormat                 string
	HealthStatus              bool
	HTTPSnippets              []string
	// http://nginx.org/en/docs/http/ngx_http_ssl_module.html
	SSLProtocols           string
	SSLPreferServerCiphers bool
	SSLCiphers             string
	SSLDHParam             string
}

// ServiceNginxConfig describes an NGINX configuration for Service LoadBalancing
type ServiceNginxConfig struct {
	Resolver  string
	Upstreams []StreamUpstream
	Servers   []StreamServer
}

// StreamUpstream describes an NGINX upstream (context stream)
// http://nginx.org/en/docs/stream/ngx_stream_upstream_module.html#upstream
// The 'hash' directive is not supported in the 'upstream' context currently.
type StreamUpstream struct {
	Name            string
	Algorithm       string
	LeastTimeMethod string
	UpstreamServers []StreamUpstreamServer
}

// StreamUpstreamServer describes a server in an NGINX upstream (context stream::upstream)
// http://nginx.org/en/docs/stream/ngx_stream_upstream_module.html#server
// The following 'server' directive parameters are ommitted, as they are only available in NGINX Plus
// - Resolve   bool
// - Service   string
// - SlowStart string
type StreamUpstreamServer struct {
	Address     string // "The address can be specified as a domain name or IP address with an obligatory port"
	Weight      string
	MaxConns    string
	MaxFails    string
	FailTimeout string
	Backup      bool
	Down        bool
}

// StreamServer describes an NGINX Server (context stream)
// http://nginx.org/en/docs/stream/ngx_stream_core_module.html#server
type StreamServer struct {
	Listen               StreamListen
	ProxyProtocol        bool
	ProxyProtocolTimeout string
	ProxyPassAddress     string
}

// StreamListen describes an NGINX server listener (context stream::server)
// http://nginx.org/en/docs/stream/ngx_stream_core_module.html#listen
type StreamListen struct {
	Address string
	Port    string
	UDP     bool
	// other fields ommitted, e.g SSL, backlog, ... so_keepalive
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

// NewStreamUpstreamWithDefaultServer creates an upstream with the default server.
// Do not initialize Algorithm or LeastTimeMethod!
func NewStreamUpstreamWithDefaultServer(name string) StreamUpstream {
	return StreamUpstream{
		Name:            name,
		UpstreamServers: []StreamUpstreamServer{StreamUpstreamServer{Address: "127.0.0.1:1234"}},
	}
}

// IsStreamUpstreamDefault - true if still default value, false otherwise.
func IsStreamUpstreamDefault(su StreamUpstream) bool {
	return reflect.DeepEqual(su, NewStreamUpstreamWithDefaultServer(su.Name))
}

// NewNginxController creates a NGINX controller
func NewNginxController(cfgType Configuration, nginxConfPath string, local bool, healthStatus bool) (*NginxController, error) {
	ngxc := NginxController{
		nginxConfdPath: path.Join(nginxConfPath, "conf.d"),
		nginxCertsPath: path.Join(nginxConfPath, "ssl"),
		local:          local,
		cfgType:        cfgType,
		mainCfg:        nil,
	}

	if !local {
		cfg := &NginxMainConfig{
			Daemon:          true,
			ErrorLogFile:    "/var/log/nginx/error.log",
			ErrorLogLevel:   "warn",
			PidFile:         "/var/run/nginx.pid",
			User:            "nginx",
			Group:           "nginx",
			WorkerProcesses: "2",
			/* For future use potentially, can be scrubbed if prefered.
			Environment: map[string]string{
				"OPENSSL_ALLOW_PROXY_CERTS": "1",
			}, */
		}
		switch cfgType {
		case ServiceCfg:
			cfg.DefaultHTTPServer = false
		case IngressCfg:
			createDir(ngxc.nginxCertsPath)
			cfg.DefaultHTTPServer = true
			cfg.HTTPContext.ServerNamesHashMaxSize = NewDefaultConfig().MainServerNamesHashMaxSize
			cfg.HTTPContext.HealthStatus = healthStatus
		}
		ngxc.mainCfg = cfg
		ngxc.UpdateMainConfigFile()
	}
	return &ngxc, nil
}

// DeleteConfiguration deletes the configuration file, which corresponds for the
// specified ingress resource / service load balancer from NGINX conf directory
func (nginx *NginxController) DeleteConfiguration(name string) {
	filename := nginx.getNginxConfigFileName(name)
	glog.V(3).Infof("deleting %v", filename)

	if !nginx.local {
		if err := os.Remove(filename); err != nil {
			glog.Warningf("Failed to delete %v: %v", filename, err)
		}
	}
}

// AddOrUpdateIngress creates or updates a file with
// the specified configuration for the specified ingress
func (nginx *NginxController) AddOrUpdateIngress(name string, config IngressNginxConfig) {
	glog.V(3).Infof("Updating NGINX configuration for Ingress Resource")
	filename := nginx.getNginxConfigFileName(name)
	nginx.templateIngress(config, filename)
}

// AddOrUpdateService creates or updates a file with the specified service config
func (nginx *NginxController) AddOrUpdateService(name string, config ServiceNginxConfig) {
	glog.V(3).Infof("Updating NGINX configuration for Service LoadBalancer")
	filename := nginx.getNginxConfigFileName(name)
	nginx.templateService(config, filename)
}

// AddOrUpdateDHParam creates the servers dhparam.pem file
func (nginx *NginxController) AddOrUpdateDHParam(dhparam string) (string, error) {
	fileName := nginx.nginxCertsPath + "/" + dhparamFilename
	if !nginx.local {
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
func (nginx *NginxController) AddOrUpdateCertAndKey(name string, cert string, key string) string {
	pemFileName := nginx.nginxCertsPath + "/" + name + ".pem"

	if !nginx.local {
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

func (nginx *NginxController) getNginxConfigFileName(name string) string {
	return path.Join(nginx.nginxConfdPath, name+".conf")
}

func (nginx *NginxController) templateIngress(config IngressNginxConfig, filename string) {
	tmpl, err := template.New("ingress.tmpl").ParseFiles("ingress.tmpl")
	if err != nil {
		glog.Infof("templateIngress: template error: %v", err)
		glog.Fatal("templateIngress: failed to parse ingress template file")
	}

	if glog.V(3) {
		glog.Infof("writing NGINX Ingress Resource configuration to %v", filename)
		tmpl.Execute(os.Stdout, config)
	}

	if !nginx.local {
		w, err := os.Create(filename)
		if err != nil {
			glog.Fatalf("failed to open %v: %v", filename, err)
		}
		defer w.Close()

		if err := tmpl.Execute(w, config); err != nil {
			glog.Fatalf("failed to write template %v", err)
		}
	}

	glog.V(3).Infof("NGINX Ingress Resource configuration file had been updated")
}

func (nginx *NginxController) templateService(config ServiceNginxConfig, filename string) {
	tmpl, err := template.New("service.tmpl").ParseFiles("service.tmpl")
	if err != nil {
		glog.Infof("templateService: template error: %v", err)
		glog.Fatal("templateService: failed to parse service template file")
	}

	if glog.V(3) {
		glog.Infof("templateService: writing NGINX Service LoadBalancer configuration to: %v", filename)
		tmpl.Execute(os.Stdout, config)
	}

	if !nginx.local {
		w, err := os.Create(filename)
		if err != nil {
			glog.Fatalf("templateService: failed to open %v: %v", filename, err)
		}
		defer w.Close()

		if err := tmpl.Execute(w, config); err != nil {
			glog.Fatalf("templateService: failed to write template %v", err)
		}
	}
	glog.V(3).Infof("templateService: NGINX Service LoadBalancer configuration file had been updated")
}

// Reload reloads NGINX
func (nginx *NginxController) Reload() error {
	if !nginx.local {
		if err := shellOut("nginx -t"); err != nil {
			return fmt.Errorf("Reload: Invalid nginx configuration detected, not reloading: %s", err)
		}
		if err := shellOut("nginx -s reload"); err != nil {
			return fmt.Errorf("Reload: Reloading NGINX failed: %s", err)
		}
	} else {
		glog.V(3).Info("Reload: Reloading nginx")
	}
	return nil
}

// Start starts NGINX
func (nginx *NginxController) Start() {
	if !nginx.local {
		if err := shellOut("nginx"); err != nil {
			glog.Fatalf("Failed to start nginx: %v", err)
		}
	} else {
		glog.V(3).Info("Starting nginx")
	}
}

func createDir(path string) {
	if err := os.Mkdir(path, os.ModeDir); err != nil {
		glog.Fatalf("Couldn't create directory %v: %v", path, err)
	}
}

func shellOut(cmd string) (err error) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	glog.V(3).Infof("executing %s", cmd)

	command := exec.Command("sh", "-c", cmd)
	command.Stdout = &stdout
	command.Stderr = &stderr

	err = command.Start()
	if err != nil {
		return fmt.Errorf("Failed to execute %v, err: %v", cmd, err)
	}

	err = command.Wait()
	if err != nil {
		return fmt.Errorf("Command %v stdout: %q\nstderr: %q\nfinished with error: %v", cmd,
			stdout.String(), stderr.String(), err)
	}
	return nil
}

// UpdateMainConfigFile update the main NGINX configuration file
func (ngxc *NginxController) UpdateMainConfigFile() {
	tmpl, err := template.New("nginx.conf.tmpl").ParseFiles("nginx.conf.tmpl")
	if err != nil {
		glog.Fatalf("Failed to parse the main config template file: %v", err)
	}

	filename := "/etc/nginx/nginx.conf"
	glog.V(3).Infof("Writing NGINX conf to %v", filename)

	if glog.V(3) {
		tmpl.Execute(os.Stdout, ngxc.mainCfg)
	}

	if !ngxc.local {
		w, err := os.Create(filename)
		if err != nil {
			glog.Fatalf("Failed to open %v: %v", filename, err)
		}
		defer w.Close()

		if err := tmpl.Execute(w, ngxc.mainCfg); err != nil {
			glog.Fatalf("Failed to write template %v", err)
		}
	}

	glog.V(3).Infof("The main NGINX configuration file had been updated")
}
