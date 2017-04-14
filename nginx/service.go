package nginx

import (
	"encoding/json"
	"reflect"

	"k8s.io/client-go/pkg/api/v1"
)

// SupportedAlgorithms - NGINX load balanacing upstream directives
// http://nginx.org/en/docs/stream/ngx_stream_upstream_module.html#upstream
// http://nginx.org/en/docs/stream/ngx_stream_upstream_module.html#least_conn
// http://nginx.org/en/docs/stream/ngx_stream_upstream_module.html#least_time
var SupportedAlgorithms = []string{
	RoundRobin,
	LeastConnections,
	LowestLatency,
}

const (
	// RoundRobin - direct traffic sequentially to the servers, default algorithm
	RoundRobin string = "round_robin"
	// LeastConnections - direct traffic to the server with the smaller number of current active connections.
	LeastConnections string = "least_conn"
	// LowestLatency - direct traffic to server with the lowest average latency and the least number of active connections.
	LowestLatency string = "least_time"
	// DefaultAlgorithm - round robin
	DefaultAlgorithm string = RoundRobin
)

// SupportedMethods - for NGINX load balanacing upstream directives leasttime:
// http://nginx.org/en/docs/stream/ngx_stream_upstream_module.html#least_time
var SupportedMethods = []string{
	Connect,
	FirstByte,
	LastByte,
	ConnectInflight,
	FirstByteInflight,
	LastByteInflight,
}

const (
	// Connect - time to connect to the upstream server is the latency measured, default method
	Connect string = "connect"
	// FirstByte - time to receive the first byte of data is the latency measured
	FirstByte string = "first_byte"
	// LastByte - time to receive the last byte of data is the latency measured
	LastByte string = "last_byte"
	// ConnectInflight - Connect timing but includes incomplete connections
	ConnectInflight string = "connect inflight"
	// FirstByteInflight - FirstByte timing but includes incomplete connections
	FirstByteInflight string = "first_byte inflight"
	// LastByteInflight - LastByte timing but includes incomplete connections
	LastByteInflight string = "last_byte inflight"
	// DefaultMethod - connect
	DefaultMethod string = Connect
)

// UpstreamTypes - service upstream pool target types
// If you're bored and need some intertaining reading about 'node' as a name:
// - https://github.com/kubernetes/kubernetes/issues/1111
var UpstreamTypes = []string{
	HostNode,
	Pod,
	ClusterIP,
}

const (
	// HostNode - upstream endpoints are the host node addresses:ports, default upstream type
	HostNode string = "node"
	// Pod -  upstream endpoints are pod addresses:ports
	Pod string = "pod"
	// ClusterIP -  upstream endpoints are cluster IP addresses:ports
	ClusterIP string = "cluster-ip"
	// DefaultUpstreamType - default service upstream pool target type
	DefaultUpstreamType string = HostNode
)

// NodeSelectionSets - node set selection
var NodeSelectionSets = []string{
	Host,
	All,
}

const (
	// Host - Upstream group is selected from only nodes that host the service's pod(s), default set
	Host string = "host"
	// NPlus1 - TODO: Upstream group is selected from the nodes that host the service's pod(s) + 1 spare
	NPlus1 string = "n+1"
	// Fixed - TODO: Upstream group is at most 'fixed' nodes where: hosts < n+1 < fixed < all
	Fixed string = "fixed"
	// All - Upstream group is made up of all nodes in the cluster
	All string = "all"
	// DefaultNodeSet - default node set
	DefaultNodeSet = Host
)

// NodeAddressType - node IP address type
var NodeAddressType = []string{
	Internal,
	External,
}

const (
	// Internal - upstream nodes IP address type is internal (assumed RFC1918), default type
	Internal string = "internal"
	// External - upstream nodes IP address type is external public or private
	External string = "external"
	// DefaultNodeAddressType - default address type
	DefaultNodeAddressType = Internal
)

// Target is a service network topology target
type Target struct {
	// ServicePort - the port that we listen on for the service's external clients
	ServicePort int
	// NodeIP - the IP address of a host/worker node
	NodeIP string
	// NodeName - the name of a host/worker node
	NodeName string
	// NodePort - the port that the host/worker node listens on for fowarding to the pod/ip:port
	NodePort int
	// PortName - the name of the port if present, or 'unnamed' otherwise
	PortName string
	// PodIP - the pods ip address
	PodIP string
	// PodPort - the port the that the pod listens on
	PodPort int
	// Protocol - TCP or UDP
	Protocol string
}

// ServiceSpec models basic Service details and the Endpoints of the services
type ServiceSpec struct {
	Service      *v1.Service
	Key          string
	Algorithm    string
	ClusterIP    string
	ConfigName   string
	UpstreamType string
	Topology     []Target
}

// ValidateAlgorithm - returns the input 'a' algorithm value iff it is a valid
// value from SupportedAlgorithms, otherwise returns default algorithm value
func ValidateAlgorithm(a string) string {
	found := false
	for _, current := range SupportedAlgorithms {
		if a == current {
			found = true
			break
		}
	}
	if !found {
		return DefaultAlgorithm
	}
	return a
}

// ValidateMethod - returns the input 'm' method value iff it is a valid value
// from SupportedMethods, otherwise returns default method value
func ValidateMethod(m string) string {
	found := false
	for _, current := range SupportedMethods {
		if m == current {
			found = true
			break
		}
	}
	if !found {
		return DefaultMethod
	}
	return m
}

// ValidateUpstreamType - returns the input 'ups' upstream type iff it is a
// valid value from UpstreamTypes, otherwise returns default upstream type
func ValidateUpstreamType(ups string) string {
	found := false
	for _, current := range UpstreamTypes {
		if ups == current {
			found = true
			break
		}
	}
	if !found {
		return DefaultUpstreamType
	}
	return ups
}

// ValidateNodeAddressType - returns the input 'set' selection iff it is a
// valid value from NodeAddressType, otherwise returns default type value
func ValidateNodeAddressType(at string) string {
	found := false
	for _, current := range NodeAddressType {
		if at == current {
			found = true
			break
		}
	}
	if !found {
		return DefaultNodeAddressType
	}
	return at
}

// ValidateNodeSet - returns the input 'set' selection iff it is a valid value
// from NodeSelectionSets, otherwise returns default set value
func ValidateNodeSet(set string) string {
	found := false
	for _, current := range NodeSelectionSets {
		if set == current {
			found = true
			break
		}
	}
	if !found {
		return DefaultNodeSet
	}
	return set
}

func (t Target) String() string {
	j, err := json.Marshal(t)
	if err != nil {
		return string("cant't marshal: " + reflect.TypeOf(t).String() + ", to json string, err: " + err.Error())
	}
	return string(j)
}

func (s ServiceSpec) String() string {
	j, err := json.Marshal(s)
	if err != nil {
		return string("cant't marshal: " + reflect.TypeOf(s).String() + ", to json string, err: " + err.Error())
	}
	return string(j)
}
