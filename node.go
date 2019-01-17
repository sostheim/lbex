package main

import (
	"errors"

	"k8s.io/client-go/pkg/api"
	v1 "k8s.io/client-go/pkg/api/v1"
)

// Node models models a k8s worker node v1.Node
type Node struct {
	Name    string
	Address NodeAddress
}

// NodeAddress models a k8s worker node address v1.NodeAddress (node.status.addresses)
type NodeAddress struct {
	Hostname   string
	ExternalIP string
	InternalIP string
}

// ValidateNodeObjectType return wether or not the given object
// is of type *api.Node or *v1.Node -> valid true, valid false otherwise
func ValidateNodeObjectType(obj interface{}) error {
	switch obj.(type) {
	case *v1.Node:
		return nil
	case *api.Node:
		return errors.New("unsupported type api.* (must be v1.*)")
	}
	return errors.New("unexpected type")
}

// GetNodeName return validated service type's name, error otherwise.
func GetNodeName(obj interface{}) (string, error) {
	node, ok := obj.(*v1.Node)
	if !ok {
		return "", errors.New("type assertion failure")
	}
	return string(node.Name), nil

}

// GetNodeNamespace return validated service type's namespace, error otherwise.
func GetNodeNamespace(obj interface{}) (string, error) {
	node, ok := obj.(*v1.Node)
	if !ok {
		return "", errors.New("type assertion failure")
	}
	return string(node.Namespace), nil
}

// GetNodePodCIDR returns the node's pod cidr ip address value as a string, or an error
func GetNodePodCIDR(obj interface{}) (string, error) {
	node, ok := obj.(*v1.Node)
	if !ok {
		return "", errors.New("type assertion failure")
	}
	return string(node.Spec.PodCIDR), nil
}

// GetNodeAddress returns the node's addresses object, or an error
func GetNodeAddress(obj interface{}) (NodeAddress, error) {
	nodeAddr := NodeAddress{}
	node, ok := obj.(*v1.Node)
	if !ok {
		return nodeAddr, errors.New("type assertion failure")
	}

	for _, addr := range node.Status.Addresses {
		switch addr.Type {
		case v1.NodeHostName:
			nodeAddr.Hostname = addr.Address
		case v1.NodeExternalIP:
			nodeAddr.ExternalIP = addr.Address
		case v1.NodeInternalIP:
			nodeAddr.InternalIP = addr.Address
		}
	}
	return nodeAddr, nil
}

// GetNodeExternalIP returns the node's external ip address value as a string, or an error
func GetNodeExternalIP(obj interface{}) (string, error) {
	addrs, err := GetNodeAddress(obj)
	if err != nil {
		return "", err
	}
	return addrs.ExternalIP, err
}

// GetNodeInternalIP returns the node's internal ip address value as a string, or an error
func GetNodeInternalIP(obj interface{}) (string, error) {
	addrs, err := GetNodeAddress(obj)
	if err != nil {
		return "", err
	}
	return addrs.InternalIP, err
}

// GetNodeHostname returns the node's internal ip address value as a string, or an error
func GetNodeHostname(obj interface{}) (string, error) {
	addrs, err := GetNodeAddress(obj)
	if err != nil {
		return "", err
	}
	return addrs.Hostname, nil
}

// IsNodeScheduleable returns the node's schedulability status, true if the
// node can schedule pods, false otherwise
func IsNodeScheduleable(obj interface{}) bool {
	node, ok := obj.(*v1.Node)
	if !ok {
		return false
	}
	return node.Spec.Unschedulable == false
}
