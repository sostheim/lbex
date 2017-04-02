package nginx

// Node models a k8s worker node's id and addresses
type Node struct {
	Name       string
	Hostname   string
	ExternalIP string
	InternalIP string
	Active     bool
}
