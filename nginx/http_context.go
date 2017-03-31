package nginx

// HTTPContext holds NGINX configuration parameters for an 'http' context
type HTTPContext struct {
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

// NewDefaultHTTPContext creates a Config with default values
func NewDefaultHTTPContext() *HTTPContext {
	return &HTTPContext{
		ServerTokens:               true,
		ProxyConnectTimeout:        "60s",
		ProxyReadTimeout:           "60s",
		ClientMaxBodySize:          "1m",
		MainServerNamesHashMaxSize: "512",
		ProxyBuffering:             true,
		HSTSMaxAge:                 2592000,
	}
}
