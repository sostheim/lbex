# NGINX Based External Kubernetes Service Load Balancer
## Overview
The Load Balancer External (LBEX) project was built to attempt to resolve a specific problem (described below).  As work progressed on LBEX, it was easy to see simple extensions that made it potentially useful for solving a larger set of problems.  The solution is built around the standard Kubernetes 1.5+ release and the community version of NGINX.  The use of NGINX in this project is entirely due to the UDP load balancing capabilities.  Providing support for both TCP and UDP protocols is a minimum requirement for being able to support general Kubernetes Services.  

In general, LBEX provides the ability to:
- Service network traffic on any Linux distribution that supports the installation of NGINX including:
    -  Running on Bare Metal 
        - as a container image or static binary
    -  Running on a Cloud Instance
        - as a container image or static binary
    -  Running as a Kubernetes managed Deployment
        - as a container image defined in a [Pod](https://kubernetes.io/docs/concepts/workloads/pods/pod/) [Spec](https://kubernetes.io/docs/api-reference/v1.6/#pod-v1-core)      
- Proxy/Load Balance traffic to: 
    - A Kubernetes ClusterIP Address and ServicePort
    - A Kubernetes Pod IP Address and Port
    - A Kubernetes worker host's IP Address and Node Port
        - Including cloud instances private or public IP Address

Note that all of the NGINX configuration is managed by LBEX.  The only requirement is that NGINX be installed and executable on the host operating system that LBEX will run on. The LBEX NGINX instance cannot be used for any other purpose.  LBEX writes, and overwrites, all of NGINX's configuration files repeatedly during normal operation.  As such it doesn't play well with any other configuration management system or human operators. 

### Discussion
A deployment of LBEX requires, at a minimum, network connectivity to both the Kubernetes API server and at least one destination address subnet.  The API server can be accessed via `kubectl proxy` for development, but this is not recommended for production deployments.  For normal operation, the standard access via the users existing kubeconfig or the API Server's endpoint is supported.  Network access must be available to at least one destination address space, either the ClusterIP Service IP address space, the Pod IP address space, or the host worker nodes IP address space (public or private).

The LBEX application can run in any environment where some reasonable combination of access to these two resources is available.    
## Using LBEX Example
The following is an example of deploying a Kubernetes service that uses LBEX for it external load balancer.  Assume that our cluster provides an [NTP servers](https://en.wikipedia.org/wiki/Network_Time_Protocol) as a Kubernetes [Deployment](https://kubernetes.io/docs/concepts/workloads/controllers/deployment/).  The example show here is actually more verbose than necessary, but that's entirely for illustration.  We'll revisit this example after a full discussion of Annotations. The following [Service](https://kubernetes.io/docs/concepts/services-networking/service/) [Specification](https://kubernetes.io/docs/api-reference/v1.6/#servicespec-v1-core) would configure LBEX for the NTP Service. 
```
apiVersion: v1
kind: Service
metadata:
  name: cluster-local-ntp
  labels:
    name: ntp-service
    app: ntp
    version: 1.0.0
  annotations:
    kubernetes.io/loadbalancer-class: loadbalancer-lbex
    loadbalancer.lbex/algorithm: round_robin
    loadbalancer.lbex/upstream-type: node
    loadbalancer.lbex/node-set: host
    loadbalancer.lbex/node-address-type: internal
spec:
  type: NodePort
  selector: 
    app: ntp-pod
  ports:
  - name: ntp-listener
    protocol: UDP
    port: 123
    nodePort: 30123
```

### How It Works
Everything should look very familiar with one obvious exception.  There is nothing out of the ordinary here with respect to the service specification, aside from the metadata object's annotations.  The annotations shown are primarily for illustration purposes (we'll discuss annotations in more detail next).  The annotations shown here have the effect of defining:
- an NGINX load balancer that accepts incoming traffic on UDP port 123
- distributes network traffic, round robin, to all Pods running the NTP service
- network traffic is delivered to the worker node's UDP node port 30123

An important consideration is that the LBEX is supplemental to any other load balancers currently in existence in the cluster.  Significantly, this in no way affects the native Kubernetes `kube-proxy` based `iptables` load balancing.  Less obvious may be the fact that any other load balancer defined for a service can operate in parallel with very limited restrictions.  

As a final note, it is very likely that a significant portion of the NGINX server configuration directives will eventually become available as configurable options via Kubernetes ConfigMaps in future releases.  

## Annotations
Kubernetes [Annotations](https://kubernetes.io/docs/concepts/overview/working-with-objects/annotations/) currently play a central role in defining how LBEX is used.  Ideally this configuration data will be migrated to Kubernetes [ConfigMaps](https://kubernetes.io/docs/user-guide/configmap/) soon.  When, and if, that happens support will be provided for all existing annotations for several subsequent versions.
### Annotation Definitions 
The annotations defined for LBEX are as follows:
<table border="1">
    <tr>
        <th>Annotations</th>
        <th>Values</th>
        <th>Default</th>
        <th>Required</th>
    </tr>
    <tr>
        <td>kubernetes.io/loadbalancer-class</td>
        <td>loadbalancer-lbex</td>
        <td>None</td>
        <td>True</td>
    </tr>
    <tr>
        <td>loadbalancer.lbex/algorithm</td>
        <td>round_robin, <br />least_conn, <br />least_time<sup>[1]</sup></td>
        <td>round_robin</td>
        <td>False</td>
    </tr>
    <tr>
        <td>loadbalancer.lbex/method<sup>[1]</sup></td>
        <td> connect, <br />first_byte, <br />last_byte, <br />connect inflight, <br />first_byte inflight, <br />last_byte inflight</td>
        <td>connect</td>
        <td>False</td>
    </tr>
    <tr>
        <td>loadbalancer.lbex/resolver</td>
        <td>The IP Address of a valid, live DNS resolver</td>
        <td>None</td>
        <td>False</td>
    </tr>
    <tr>
        <td>loadbalancer.lbex/upstream-type</td>
        <td>node, <br />pod, <br />cluster-ip</td>
        <td>node</td>
        <td>False</td>
    </tr>
    <tr>
        <td>loadbalancer.lbex/node-set</td>
        <td>host, all</td>
        <td>host</td>
        <td>False</td>
    </tr>
    <tr>
        <td>loadbalancer.lbex/node-address-type</td>
        <td>internal, <br />external</td>
        <td>internal</td>
        <td>False</td>
    </tr>
</table>
    [1] The least_time load balancing method is only available in NGINX Plus

### Annotation Descriptions 
The only mandatory value that must be present for LBEX to serve traffic for the intended Kubernetes Service is `kubernetes.io/loadbalancer-class`.  Every other annotation has either a sensible default or is optional.

<b>loadbalancer.lbex/algorithm</b> - Defaults to round robin, but can also be set to least connections.  The option to select leas time (lowest measured time) is supported, but can only be used with NGINX Plus.

<b>loadbalancer.lbex/method</b> - Method is a supplemental argument to the least_time directive.  Similarly, it is supported in LBEX but requires NGINX Plus to function.  See reference: [least_time](http://nginx.org/en/docs/stream/ngx_stream_upstream_module.html#least_time).

<b>loadbalancer.lbex/resolver</b> - Configures name servers used to resolve names of upstream servers into addresses. See reference: [resolver](https://nginx.org/en/docs/stream/ngx_stream_core_module.html#resolver).

<b>loadbalancer.lbex/upstream-type</b> - Upstream type indicates the type of the backend service addresses to direct to.  The default, `node`, directs load balanced traffic to the Kubernetes host worker node and node port.  Alternatively, `pod` directs traffic to the Kubernetes Pod and its' corresponding port.  Finally, `cluster-ip' directs traffic to the Kubernetes Service's ClusterIP.

The final two annotations are only read if, and only if, `loadbalancer.lbex/upstream-type=node`. 

<b>loadbalancer.lbex/node-set</b> - Selects the set of Kubernetes host worker nodes to add to the upstream for the load balancer.  The default `host` ensures that traffic is only directed to noes that are actively running a copy of the services backend pod.  By contrast, `all` will direct traffic to any available Kubernetes worker node.

<b>loadbalancer.lbex/node-address-type</b> - Determines whether to direct load balanced traffic to the node's `internal` private IP address (default), or its' `external` public IP address. 

### Annotation Selection
It is incumbent on the service designer to make sensible selections for annotation values.  For example, it makes no sense to select a node address type of `external` if the worker nodes in the Kubernetes cluster haven't been created with external IP addresses.  It would also be off to try to select an upstream type of `cluster-ip` if 1) the service doesn't provide one, 2) LBEX is not running as a Pod inside the Kubernetes the cluster.  By definition a cluster IP address is only accessible to members of the cluster.

## Using LBEX Example - Revisited
Returning to the pervious example, here is the updated version that takes advantage of the default values for all but the one required annotation.  As before, the following Service Specification would configure LBEX for the NTP Service. 
```
apiVersion: v1
kind: Service
metadata:
  name: cluster-local-ntp
  labels:
    name: ntp-service
    app: ntp
    version: 1.0.0
  annotations:
    kubernetes.io/loadbalancer-class: loadbalancer-lbex
 spec:
  type: NodePort
  selector: 
    app: ntp-pod
  ports:
  - name: ntp-listener
    protocol: UDP
    port: 123
    nodePort: 30123
```

So, by taking advantage of several sensible defaults, the service's definition is exactly as it would be were it not using LBEX aside from the addition of a one line annotation.

## NGINX Prerequisites

For TCP and UDP load balancing to work, the NGINX image must be built with the `--with-stream` configuration flag to load/enable the required stream processing modules.  In most cases the [NGINX Official Repository](https://hub.docker.com/_/nginx/) 'latest' tagged image will include the stream modules by default.  The easiest way to be certain that the modules are included is to dump the configuration and check for their presence.

For example, running the following command against the `nginx:latest` image shows the following (line breaks added for clarity)

    $ docker run -t nginx:latest nginx -V
    nginx version: nginx/1.11.10
    built by gcc 4.9.2 (Debian 4.9.2-10) 
    built with OpenSSL 1.0.1t  3 May 2016
    TLS SNI support enabled
    configure arguments: --prefix=/etc/nginx 
                --sbin-path=/usr/sbin/nginx 
                --modules-path=/usr/lib/nginx/modules 
                --conf-path=/etc/nginx/nginx.conf 
                --error-log-path=/var/log/nginx/error.log 
                --http-log-path=/var/log/nginx/access.log 
                --pid-path=/var/run/nginx.pid 
                --lock-path=/var/run/nginx.lock 
                --http-client-body-temp-path=/var/cache/nginx/client_temp 
                --http-proxy-temp-path=/var/cache/nginx/proxy_temp 
                --http-fastcgi-temp-path=/var/cache/nginx/fastcgi_temp 
                --http-uwsgi-temp-path=/var/cache/nginx/uwsgi_temp 
                --http-scgi-temp-path=/var/cache/nginx/scgi_temp 
                --user=nginx 
                --group=nginx 
                --with-compat 
                --with-file-aio 
                --with-threads 
                --with-http_addition_module 
                --with-http_auth_request_module 
                --with-http_dav_module 
                --with-http_flv_module 
                --with-http_gunzip_module 
                --with-http_gzip_static_module 
                --with-http_mp4_module 
                --with-http_random_index_module 
                --with-http_realip_module 
                --with-http_secure_link_module 
                --with-http_slice_module 
                --with-http_ssl_module 
                --with-http_stub_status_module 
                --with-http_sub_module 
                --with-http_v2_module 
                --with-mail 
                --with-mail_ssl_module 
                --with-stream 
                --with-stream_realip_module 
                --with-stream_ssl_module 
                --with-stream_ssl_preread_module 
                --with-cc-opt='-g -O2 -fstack-protector-strong -Wformat -Werror=format-security -Wp,-D_FORTIFY_SOURCE=2 -fPIC' 
                --with-ld-opt='-Wl,-z,relro -Wl,-z,now -Wl,
                --as-needed -pie'

As you can see several stream modules are included in the NGINX build configuration. 


## Running LBEX
As noted [above](##overview), LBEX can run as either a container image under a container runtime, or in a Kubernetes Pod.  Regardless of the runtime environment, LBEX has a number of command line options that define how it operates.

```
$ ./lbex --help
Usage of ./lbex:
      --alsologtostderr                  log to standard error as well as files
      --health-check                     Enable lbex-only health check (default true)
      --health-port int                  Lbex health check port (default 7331)
      --kubeconfig string                absolute path to the kubeconfig file
      --log_backtrace_at traceLocation   when logging hits line file:N, emit a stack trace (default :0)
      --log_dir string                   If non-empty, write log files in this directory
      --logtostderr                      log to standard error instead of files
      --proxy string                     kubctl proxy server running at the given url
      --service-name string              Provide Load Balancing for the specified service.
      --stderrthreshold severity         logs at or above this threshold go to stderr (default 2)
  -v, --v Level                          log level for V logs
      --version                          Display version info
      --vmodule moduleSpec               comma-separated list of pattern=N settings for file-filtered logging
```
Without going in to an explanation of all of the parameters, many of which should have sufficient explanation in the help provided, of particular interest to controlling the operation of LBEX are the following:
<b>--health-check</b> - Defaults to true, but may be disabled by passing a value of false.  Allows external service monitors to check the health of 'lbex' itself.  
<b>--health-port</b> - Defaults to 7331, but may be set to any valid port number value.
<b>--kubeconfig</b> - Use the available kubeconfig for credentialed access to the cluster.
<b>--proxy</b> - Use the 'kubectl proxy' URL for access to the cluster. See for example [using kubectl proxy](https://kubernetes.io/docs/concepts/cluster-administration/access-cluster/#using-kubectl-proxy).
<b>--service-name</b> - Provide load balancing for service identified by the flag *only*.  

Note that the health check service is an HTTP service that returns simply the string `healthy` as the body, and a `200` HTTP response code if the service is running.  For example:
```
$ curl http://10.150.0.2:7331/ -w "HTTP Response Code: %{http_code}\n"
healthy
HTTP Response Code: 200
```

There is an implied default not noted here that relates to accessing the Kubernetes cluster.  LBEX will attempt to establish credentialed cluster access via the following methods listed in priority order:
1. If `--proxy string` is provided use it, then it is used without attempting methods 2 and 3
2. If `--kubeconfig string` is provided, then it is used without attempting method 3
3. If neither method is specified, then assume we are running inside a Kubernetes Pod and use the associated Service Account.  See detailed information [here](https://kubernetes.io/docs/tasks/configure-pod-container/configure-service-account/.)

Also implied in the ordering above is that only one attempt is made to access the cluster.  Regardless of which method is used, it is the only method attempted.  If the selected method fails, then LBEX terminates immediately without attempting any other access.  Therefore, it is unnecessary to specify more than one access method.

Finally, the `--service-name` option, allows us to provide a 1:1 mapping of load balancer's to services, should we desire to do so.  The identified service must still provide the required annotation, `kubernetes.io/loadbalancer-class: loadbalancer-lbex`, but no other services will have their traffic managed by this instance of LBEX regardless of wether or not they supply the identifying annotation.  

## Installation on Google Cloud

The [bash scripts](bin) for installation on Google Cloud are provided under the bin folder.

Run either: `./gce-up.sh --help`, or: `./gce-down.sh --help` to see the list of supported options:

```
Usage:

gce-up.sh or gce-down.sh [flags]

Flags are:
-c|--cidr            - CIDR range for LBEX instance IP addresses, big enough for at least 'num-autoscale' IPs. Should not clash with GKE cluster ip space. Default is 10.150.0.0/28.
-h|--help            - Show this help.

-i|--project         - Google project id. Required.
-m|--cluster-name    - Target GKE cluster name. Required for gce-up.sh.
-n|--name            - base name for all lbex resource. Required.
-p|--health-port     - LBEX health check port. Default is 7331.
-r|--region          - GCE region name. Default is us-central1.
-s|--num-autoscale   - Maximum number of auto-scaled LBEX instances. Default is 10.
-t|--cluster-network - GCE network name of the GKE cluster. Must be a custom type network. Required
-z|--cluster-zone    - Target GKE cluster primary zone. Default is us-central1-a.

For example:
gce-up.sh --name mylbex --project k8s-work --cidr 10.128.0.0/28 --region us-central1 --cluster-name my-k8s-cluster --num-autoscale 10 --cluster-zone us-central1-a --cluster-network my-cluster-network --health-port 1337
or
gce-down.sh --name mylbex --region us-central1 --project k8s-work
```

### Google Cloud Prerequisites

For LBEX to work with your GKE cluster, the cluster must have been created with non-default, non-automatic [subnet network](https://cloud.google.com/compute/docs/subnetworks#subnet_network). You will need to provide your cluster's name, network name and a CIDR block that does not conflict with any existing CIDR blocks in your GKE cluster's subnet network.

### Running the Scripts

For example, given a GKE cluster named `mycluster` with primary zone of `us-central1-a`; created using a subnet network `mynetwork` with one `us-central1` subnet with a CIDR of `10.128.0.0/20` in a Google Project named `myproject`:

```
./gce-up.sh \
  --name my-lbex \
  --project myproject \
  --region us-central1 \
  --cidr 10.150.0.0/28 \
  --cluster-name mycluster \
  --cluster-zone us-central1-a \
  --cluster-network mynetwork 
```
This will create an autoscaling managed instance group in `us-central1`, that will scale to max `10`, minimum `2` instances, auto-heal based on lbex health check at default port `7331`; with CIDR `10.150.0.0/28`, monitoring the API server of GKE cluster `mycluster` for services to proivde external load balancing for.

## Motivation
A very specific use case arises for Google Container Engine (GKE) base Kubernetes services that require an external load balancer, but not a public IP address.  That is, services that need to be exposed to RFC1918 address spaces, but that address space is neither part of the Cluster's IP address space, or the [GCP Subnet Network](https://cloud.google.com/compute/docs/networking#subnet_network) Auto [IP Ranges](https://cloud.google.com/compute/docs/networking#ip_ranges).  This is particularly challenging when connecting to GCP via [Google Cloud VPN](https://cloud.google.com/compute/docs/vpn/overview), where the on premise peer network side of the VPN is also an RFC1918 10/8 network space.  This configuration, in and of itself, presents certain challenges described here: [GCI IP Tables Configuration](https://github.com/samsung-cnct/gci-iptables-conf-agent).   Once the two networks are interconnected, there is still the issue of communicating with the GCP region's private IP subnet range, and further being able to reach exposed Kubernetes services in the Kubernetes Cluster CIDR range.

There were several attempts at solving this problem with a combination of various [Google Cloud Load Balancing](https://cloud.google.com/load-balancing/) components, including using the [GCP Internal Load Balancer](https://cloud.google.com/compute/docs/load-balancing/internal/) and following the model provided by the [Internal Load Balancing using HAProxy on Google Compute Engine](https://cloud.google.com/solutions/internal-load-balancing-haproxy) example.

In the end, the best solution we were able to arrive at was 1) not dynamic and 2) exposed a high order ephemeral port.  
1. This meant that, since the GCP Internal LB solution had to have stable endpoints, there was an external requirement to ensure that service specifications confirmed to certain constraints.  Conversely, anytime a service configuration change was made, or a new service introduce in to the environment, a corresponding LB had to created and/or updated.
2. This was first and foremost unsightly and awkward to manage.  Over time it was the leaky abstraction that was the most bothersome and provided extra motivation to move forward with a better solution.
 
Finally, there are challenges to automating all these things as well.  None of them are insurmountable by any means, but when justifying the engineering effort to automate operations you prefer it to be for the right solution.  