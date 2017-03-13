# Cloud Based NGINX External Service Load Balancer (lbex)

A very specific use case arises for Google Conatiner Engine (GKE) base Kubernetes services that require an external loadbalancer, but not a public IP address.  That is, services that need to be exposed to RFC1918 address spaces, but that address space is neither part of the Cluster IP address space, or the [GCP Subnet Network](https://cloud.google.com/compute/docs/networking#subnet_network) Auto [IP Ranges](https://cloud.google.com/compute/docs/networking#ip_ranges).  Specifically, when connecting to GCP via CloudVPN, where the onpremise side of the VPN is an RFC1918 10/8 network space that must communicate with the region's private IP subnet range to be able to reach exposed Kubernetes servcies. 

## Overview

More to come...

## Example

Do stuff...

