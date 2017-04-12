#!/bin/bash -
#title           :gce-up.sh
#description     :bring up an LBEX group
#author          :Samsung SDSRA
#==============================================================================
set -o errexit
set -o nounset
set -o pipefail

# pull in utils
my_dir=$(dirname "${BASH_SOURCE}")
source "${my_dir}/utils.sh"

if [ -z "${LBEX_CLUSTER_NAME+x}" ]; then
  show_help
  exit 1
fi

if [ -z "${LBEX_REGION+x}" ]; then
  LBEX_REGION='us-central1'
  inf "Using '${LBEX_REGION}' as region"
fi

if [ -z "${LBEX_CIDR+x}" ]; then
  LBEX_CIDR='10.128.0.0/28'
  inf "Using '${LBEX_CIDR}' as CIDR"
fi

if [ -z "${LBEX_CLUSTER_ZONE+x}" ]; then
  LBEX_CLUSTER_ZONE="${LBEX_ZONE}"
  inf "Using '${LBEX_CLUSTER_ZONE}' as cluster zone"
fi

if [ -z "${LBEX_PORTS+x}" ]; then
  LBEX_PORTS="80,443"
  inf "Using '${LBEX_PORTS}' as cluster zone"
fi

if [ -z "${LBEX_MAX_AUTOSCALE+x}" ]; then
  LBEX_MAX_AUTOSCALE=10
  inf "Using '${LBEX_MAX_AUTOSCALE}' as max number of LBEX instances"
fi

if [ -z "${LBEX_HEALTH_PORT+x}" ]; then
  LBEX_HEALTH_PORT=7331
  inf "Using '${LBEX_HEALTH_PORT}' as health check port"
fi

# create a custom network and subnet
gcloud compute networks create \
  ${LBEX_BASE_NAME}-network \
  --description="Network for ${LBEX_BASE_NAME} LBEX instances" \
  --mode=custom \
  --project=${LBEX_PROJECT}

gcloud compute networks subnets create \
  ${LBEX_BASE_NAME}-subnetwork \
  --description="Sub-network for ${LBEX_BASE_NAME}-LBEX instances" \
  --network="${LBEX_BASE_NAME}-network" \
  --range=${LBEX_CIDR} \
  --region=${LBEX_REGION} \
  --project=${LBEX_PROJECT}

# add all-firewall rule
gcloud compute firewall-rules create \
  ${LBEX_BASE_NAME}-all-traffic \
  --network="${LBEX_BASE_NAME}-network" \
  --allow tcp,udp,icmp \
  --source-ranges=0.0.0.0/0 \
  --project=${LBEX_PROJECT}


# create a string of '-p port:port' pairs for docker port publishing
DOCKER_PORTS=""
for i in ${LBEX_PORTS//,/ }
do
  port_array=(${i//// })
  if [[ ${#port_array[@]} > 1 ]]; then
    port_pair="${port_array[0]}:${port_array[0]}/${port_array[1]}"
  else
    port_pair="${i}:${i}"
  fi

  DOCKER_PORTS="${DOCKER_PORTS}-p ${port_pair} " 
done

# create 'templated' cloud init for the instance template
TEMPDIR=$(mktemp -d "${TMPDIR:-/tmp/}$(basename 0).XXXXXXXXXXXX")
cat << EOF > "${TEMPDIR}/cloud-init"
#cloud-config
write_files:
- path: /etc/systemd/system/lbex.service
  permissions: 0644
  owner: root
  content: |
    [Unit]
    Description=Start lbex container
    After=status.service

    [Service]
    ExecStartPre=/bin/mkdir -p /var/kubeconfig
    ExecStartPre=/usr/bin/toolbox /google-cloud-sdk/bin/gcloud components install kubectl -q
    ExecStartPre=/usr/bin/toolbox --bind=/var/kubeconfig:/root/.kube /google-cloud-sdk/bin/gcloud container clusters get-credentials ${LBEX_CLUSTER_NAME} --zone ${LBEX_CLUSTER_ZONE} --project ${LBEX_PROJECT}
    ExecStart=/usr/bin/docker run --rm --name=lbex --volume /var/kubeconfig:/kubeconfig ${DOCKER_PORTS} quay.io/samsung_cnct/lbex:latest --kubeconfig /kubeconfig/config
    ExecStop=/usr/bin/docker stop lbex
    ExecStopPost=/usr/bin/docker rm lbex
- path: /etc/systemd/system/status.socket
  permissions: 0644
  owner: root
  content: |
    [Unit]
    Description=Monitor Socket

    [Socket]
    ListenStream=${LBEX_HEALTH_PORT}
    Backlog=1
    MaxConnections=10
    Accept=yes

    [Install]
    WantedBy=sockets.target
- path: /etc/systemd/system/status.service
  permissions: 0644
  owner: root
  content: |
    [Unit]
    Description=monitor service
    Requires=status.socket
    After=syslog.target network.target 

    [Service]
    Type=forking
    ExecStart=/var/status.sh

    [Install]
    WantedBy=multi-user.target
    Also=status.socket
- path: /var/status.sh
  permissions: 0744
  owner: root
  content: |
    #!/bin/sh
    # fd 3 is what systemd gave us

    while true ; do
       sed -e 's/\r\$//' | IFS=":" read header line <&3
       [ "\$header" == "Host" ] && host="\$header:\$line"
       [ -z "\$header" ] && break
    done

    RUNNING=\$(docker inspect --format="{{.State.Running}}" lbex 2> /dev/null)

    if [ \$? -eq 1 ]; then
      cat >&3 <<EOF
    HTTP/1.0 404 Not Found
    EOF
    elif [ "\$RUNNING" == "false" ]; then
      cat >&3 <<EOF
    HTTP/1.0 404 Not Found
    EOF
    else
      cat >&3 <<EOF
    HTTP/1.0 200 OK
    EOF       
    fi

    cat >&3 <<EOF
    Content-Type: text/html
    Content-Length: 43
    Date: \$(date -R)
    EOF

    [ -n "\$host" ] && echo \$host | cat >&3

    cat >&3 <<EOF
    <html>
    <body>
    <h1>LBEX</h1>
    </html>
    </body>
    EOF

runcmd:
- systemctl daemon-reload
- systemctl start status.service
- systemctl start lbex.service
EOF

# create an instance template
gcloud compute instance-templates create \
  ${LBEX_BASE_NAME}-instance \
  --description="${LBEX_BASE_NAME} instance template" \
  --machine-type=n1-standard-1 \
  --metadata-from-file "user-data=${TEMPDIR}/cloud-init" \
  --network="${LBEX_BASE_NAME}-network" \
  --subnet="${LBEX_BASE_NAME}-subnetwork" \
  --region=${LBEX_REGION} \
  --image-project=cos-cloud \
  --image-family=cos-stable \
  --scopes=compute-rw,cloud-platform,storage-full,logging-write,monitoring \
  --project=${LBEX_PROJECT}

# create a managed group
gcloud compute instance-groups managed create \
  ${LBEX_BASE_NAME}-instance-group \
  --template="${LBEX_BASE_NAME}-instance" \
  --size=2 \
  --base-instance-name="${LBEX_BASE_NAME}" \
  --description="${LBEX_BASE_NAME} LBEX managed instance group" \
  --region=${LBEX_REGION} \
  --project=${LBEX_PROJECT}

# set autoscaling
gcloud compute instance-groups managed set-autoscaling \
  ${LBEX_BASE_NAME}-instance-group \
  --max-num-replicas=${LBEX_MAX_AUTOSCALE} \
  --description="${LBEX_BASE_NAME}-instance-group autoscaler" \
  --min-num-replicas=2 \
  --scale-based-on-cpu \
  --target-cpu-utilization=0.4 \
  --region=${LBEX_REGION} \
  --project=${LBEX_PROJECT}

# create a healthcheck and set autohealing
gcloud compute http-health-checks create \
  ${LBEX_BASE_NAME}-healthcheck \
  --description="${LBEX_BASE_NAME} health checker" \
  --check-interval=5s \
  --healthy-threshold=2 \
  --port=${LBEX_HEALTH_PORT} \
  --timeout=5s \
  --unhealthy-threshold=2 \
  --project=${LBEX_PROJECT}

gcloud beta compute instance-groups managed set-autohealing \
  ${LBEX_BASE_NAME}-instance-group \
  --initial-delay=180 \
  --http-health-check=${LBEX_BASE_NAME}-healthcheck \
  --region=${LBEX_REGION} \
  --project=${LBEX_PROJECT}

