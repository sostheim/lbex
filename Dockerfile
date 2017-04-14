#
# Copyright © 2016 Samsung CNCT
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License. 
#
# Dockerfile - External LoadBalancer (lbex).
#

FROM nginx:1.11.10
MAINTAINER Rick Sostheim
LABEL vendor="Samsung CNCT"

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
