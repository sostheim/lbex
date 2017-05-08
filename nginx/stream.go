package nginx

import (
	"encoding/json"
	"os"
	"path"
	"reflect"
	"text/template"

	"github.com/golang/glog"
)

// StreamNginxConfig describes an NGINX Stream configuration primarily for Service LoadBalancing
type StreamNginxConfig struct {
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
// The following 'server' directive parameters are omitted, as they are only available in NGINX Plus
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
	// other fields omitted, e.g SSL, backlog, ... so_keepalive
}

// NewStreamUpstreamWithDefaultServer creates an upstream with the default server.
// Do not initialize Algorithm or LeastTimeMethod!
func NewStreamUpstreamWithDefaultServer(name string) StreamUpstream {
	return StreamUpstream{
		Name:            name,
		UpstreamServers: []StreamUpstreamServer{{Address: "127.0.0.1:1234"}},
	}
}

// IsStreamUpstreamDefault - true if still default value, false otherwise.
func IsStreamUpstreamDefault(su StreamUpstream) bool {
	return reflect.DeepEqual(su, NewStreamUpstreamWithDefaultServer(su.Name))
}

// DeleteStreamConfiguration deletes the configuration file, which corresponds to the
// specified stream load balancer from NGINX conf directory
func (ngxc *NginxController) DeleteStreamConfiguration(name string) {
	filename := ngxc.getStreamConfigFileName(name)

	if ngxc.cfgType != LocalCfg {
		// Many services are checked for existence, regarless of whether or not
		// we have a configuration for that service.
		if _, err := os.Stat(filename); os.IsNotExist(err) {
			return
		}
		glog.V(2).Infof("deleting %v", filename)
		if err := os.Remove(filename); err != nil {
			glog.Warningf("Failed to delete %v: %v", filename, err)
		}
	}
}

// AddOrUpdateStream creates or updates a file with the specified stream config
func (ngxc *NginxController) AddOrUpdateStream(name string, config StreamNginxConfig) {
	filename := ngxc.getStreamConfigFileName(name)
	ngxc.templateStream(config, filename)
}

func (ngxc *NginxController) getStreamConfigFileName(name string) string {
	return path.Join(ngxc.nginxConfdPath, name+".stream.conf")
}

func (ngxc *NginxController) templateStream(config StreamNginxConfig, filename string) {
	tmpl, err := template.New("stream.tmpl").ParseFiles("stream.tmpl")
	if err != nil {
		glog.Fatalf("failed to parse stream template file: %v", err)
	}

	if glog.V(2) {
		glog.Infof("writing NGINX stream configuration to: %v", filename)
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

func (s StreamNginxConfig) String() string {
	j, err := json.Marshal(s)
	if err != nil {
		return string("cant't marshal: " + reflect.TypeOf(s).String() + ", to json string, err: " + err.Error())
	}
	return string(j)
}

func (s StreamUpstream) String() string {
	j, err := json.Marshal(s)
	if err != nil {
		return string("cant't marshal: " + reflect.TypeOf(s).String() + ", to json string, err: " + err.Error())
	}
	return string(j)
}

func (s StreamUpstreamServer) String() string {
	j, err := json.Marshal(s)
	if err != nil {
		return string("cant't marshal: " + reflect.TypeOf(s).String() + ", to json string, err: " + err.Error())
	}
	return string(j)
}

func (s StreamServer) String() string {
	j, err := json.Marshal(s)
	if err != nil {
		return string("cant't marshal: " + reflect.TypeOf(s).String() + ", to json string, err: " + err.Error())
	}
	return string(j)
}

func (s StreamListen) String() string {
	j, err := json.Marshal(s)
	if err != nil {
		return string("cant't marshal: " + reflect.TypeOf(s).String() + ", to json string, err: " + err.Error())
	}
	return string(j)
}
