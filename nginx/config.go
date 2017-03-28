package nginx

type Configuration uint8

const (
	IngressCfg = Configuration(iota)
	ServiceCfg
)

// Config holds NGINX configuration parameters
type Config struct {
	// Context: main directives
	DefaultHTTPServer bool
	Daemon            bool
	ErrorLogFile      string
	ErrorLogLevel     string
	Environment       map[string]string
	LockFile          string
	PidFile           string
	User              string
	Group             string
	WorkerPriority    string
	// TODO: This needs to be a ConfigMap entry or CLI flag so that we can make
	//       it a function of the number of CPUs/vCPUs, and configure the POD
	//       resource limits propotionally for the scheduler.  For now this
	//       *should probably not* be set to 'auto'
	WorkerProcesses  string
	WorkingDirectory string

	// Context: events directives
	AcceptMutex       bool
	AcceptMutexDelay  string
	MultiAccept       bool
	WorkerConnections string

	// Context: http directives
	LocationSnippets              []string
	ServerSnippets                []string
	ServerTokens                  bool
	ProxyConnectTimeout           string
	ProxyReadTimeout              string
	ClientMaxBodySize             string
	HTTP2                         bool
	RedirectToHTTPS               bool
	MainHTTPSnippets              []string
	MainServerNamesHashBucketSize string
	MainServerNamesHashMaxSize    string
	MainLogFormat                 string
	ProxyBuffering                bool
	ProxyBuffers                  string
	ProxyBufferSize               string
	ProxyMaxTempFileSize          string
	ProxyProtocol                 bool
	ProxyHideHeaders              []string
	ProxyPassHeaders              []string
	HSTS                          bool
	HSTSMaxAge                    int64
	HSTSIncludeSubdomains         bool

	// http://nginx.org/en/docs/http/ngx_http_realip_module.html
	RealIPHeader    string
	SetRealIPFrom   []string
	RealIPRecursive bool

	// http://nginx.org/en/docs/http/ngx_http_ssl_module.html
	MainServerSSLProtocols           string
	MainServerSSLPreferServerCiphers bool
	MainServerSSLCiphers             string
	MainServerSSLDHParam             string
}

// NewDefaultConfig creates a Config with default values
func NewDefaultConfig(cnfType Configuration) *Config {
	config := &Config{
		Daemon:            true,
		ErrorLogFile:      "/var/log/nginx/error.log",
		ErrorLogLevel:     "warn",
		PidFile:           "/var/run/nginx.pid",
		WorkerProcesses:   "2",
		WorkerConnections: "1024",
		User:              "nginx",
	}
	switch cnfType {
	case IngressCfg:
		newDefaultIngressConfig(config)
	case ServiceCfg:
		newDefaultServiceConfig(config)
	}
	return config
}

func newDefaultIngressConfig(config *Config) {
	config.DefaultHTTPServer = true
	config.ServerTokens = true
	config.ProxyConnectTimeout = "60s"
	config.ProxyReadTimeout = "60s"
	config.ClientMaxBodySize = "1m"
	config.MainServerNamesHashMaxSize = "512"
	config.ProxyBuffering = true
	config.HSTSMaxAge = 2592000
}

func newDefaultServiceConfig(config *Config) {
	config.DefaultHTTPServer = false
}
