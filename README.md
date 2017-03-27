# Cloud Based NGINX External Service Load Balancer (lbex)

A very specific use case arises for Google Conatiner Engine (GKE) base Kubernetes services that require an external loadbalancer, but not a public IP address.  That is, services that need to be exposed to RFC1918 address spaces, but that address space is neither part of the Cluster IP address space, or the [GCP Subnet Network](https://cloud.google.com/compute/docs/networking#subnet_network) Auto [IP Ranges](https://cloud.google.com/compute/docs/networking#ip_ranges).  Specifically, when connecting to GCP via CloudVPN, where the onpremise side of the VPN is an RFC1918 10/8 network space that must communicate with the region's private IP subnet range to be able to reach exposed Kubernetes servcies. 

## Overview

More to come...

## Example

Do stuff...

## NGINX Prerequisites

For TCP and UDP load balancing to work, the NGINX image must be buld with the `--with-stream` configuration flag to load/enable the required stream processing moduels.  In most cases the [NGINX Official Reposiory](https://hub.docker.com/_/nginx/) 'latest' tagged image will include the stream modules by default.  The easiest way to be certain that the moduels are included is to dump the configuration and check for their presence.

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
