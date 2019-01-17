#
# Dockerfile - External LoadBalancer (lbex).
#

FROM nginx:1.11.10
LABEL vendor="sostheim"

# forward nginx access and error logs to stdout and stderr of the ingress
# controller process
RUN ln -sf /proc/1/fd/1 /var/log/nginx/access.log \
	&& ln -sf /proc/1/fd/2 /var/log/nginx/error.log

RUN rm /etc/nginx/conf.d/*

# for troubleshooting, can be removed for production deployment
RUN apt update
RUN apt install netcat-openbsd net-tools iproute2 -y

COPY build/linux_amd64/lbex nginx/http.tmpl nginx/stream.tmpl nginx/nginx.conf.tmpl /

ENTRYPOINT ["/lbex"]
