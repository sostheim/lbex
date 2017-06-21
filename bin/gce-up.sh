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

if [ -z "${LBEX_CLUSTER_NETWORK+x}" ]; then
  show_help
  exit 1
fi

if [ -z "${LBEX_INGRESS_CIDR+x}" ]; then 
  LBEX_INGRESS_CIDR="0.0.0.0/0"
  inf "Using '${LBEX_INGRESS_CIDR}' as lbex ingress traffic CIDR"
fi

if [ -z "${LBEX_REGION+x}" ]; then
  LBEX_REGION='us-central1'
  inf "Using '${LBEX_REGION}' as region"
fi

if [ -z "${LBEX_CIDR+x}" ]; then
  LBEX_CIDR='10.150.0.0/28'
  inf "Using '${LBEX_CIDR}' as CIDR"
fi

if [ -z "${LBEX_CLUSTER_ZONE+x}" ]; then
  LBEX_CLUSTER_ZONE="${LBEX_ZONE}"
  inf "Using '${LBEX_CLUSTER_ZONE}' as cluster zone"
fi

if [ -z "${LBEX_MAX_AUTOSCALE+x}" ]; then
  LBEX_MAX_AUTOSCALE=10
  inf "Using '${LBEX_MAX_AUTOSCALE}' as max number of LBEX instances"
fi

if [ -z "${LBEX_HEALTH_PORT+x}" ]; then
  LBEX_HEALTH_PORT=7331
  inf "Using '${LBEX_HEALTH_PORT}' as health check port"
fi

inf "Creating subnet ${LBEX_BASE_NAME}-subnetwork of ${LBEX_CLUSTER_NETWORK}"
gcloud compute networks subnets create \
  ${LBEX_BASE_NAME}-subnetwork \
  --description="Sub-network for ${LBEX_BASE_NAME}-LBEX instances" \
  --network="${LBEX_CLUSTER_NETWORK}" \
  --range=${LBEX_CIDR} \
  --region=${LBEX_REGION} \
  --project=${LBEX_PROJECT}

# add firewall rules
inf "Creating firewall rule ${LBEX_BASE_NAME}-lbex-traffic"
gcloud compute firewall-rules create \
  ${LBEX_BASE_NAME}-all-traffic \
  --description "Firewall rule for traffic entering ${LBEX_BASE_NAME} lbex cluster" \
  --network="${LBEX_CLUSTER_NETWORK}" \
  --allow tcp,udp,icmp \
  --source-ranges=${LBEX_INGRESS_CIDR} \
  --target-tags=${LBEX_BASE_NAME} \
  --project=${LBEX_PROJECT}
inf "Creating firewall rule ${LBEX_BASE_NAME}-cluster-traffic"
gcloud compute firewall-rules create \
  ${LBEX_BASE_NAME}-cluster-traffic \
  --description "Firewall rule for traffic between ${LBEX_BASE_NAME} lbex cluster and ${LBEX_CLUSTER_NAME} GKE cluster" \
  --network="${LBEX_CLUSTER_NETWORK}" \
  --allow tcp:1-65535,udp:1-65535,icmp \
  --source-ranges=${LBEX_CIDR} \
  --project=${LBEX_PROJECT}

# create 'templated' cloud init for the instance template
TEMPDIR=$(mktemp -d "${TMPDIR:-/tmp/}$(basename 0).XXXXXXXXXXXX")
cat << EOF > "${TEMPDIR}/cloud-init"
#cloud-config
write_files:
- path: /etc/systemd/system/nginx.service
  permissions: 0644
  owner: root
  content: |
    [Unit]
    Description=The NGINX HTTP and reverse proxy server
    After=syslog.target network.target remote-fs.target nss-lookup.target

    [Service]
    Type=forking
    PIDFile=/run/nginx.pid
    ExecStartPre=/bin/bash -c "echo \"deb http://nginx.org/packages/ubuntu/ xenial nginx\" > /etc/apt/sources.list.d/nginx.list"
    ExecStartPre=/bin/bash -c "echo \"deb-src http://nginx.org/packages/ubuntu/ xenial nginx\" >> /etc/apt/sources.list.d/nginx.list"
    ExecStartPre=/usr/bin/apt-key adv --keyserver keyserver.ubuntu.com --recv-keys ABF5BD827BD9BF62
    ExecStartPre=/usr/bin/apt-get update -y
    ExecStartPre=/usr/bin/apt-get install nginx=1.12.0-1~xenial -y
    ExecStartPre=/usr/sbin/nginx -t
    ExecStart=/usr/sbin/nginx
    ExecReload=/bin/kill -s HUP \$MAINPID
    ExecStop=/bin/kill -s QUIT \$MAINPID
    PrivateTmp=true

    [Install]
    WantedBy=multi-user.target

- path: /etc/systemd/system/lbex.service
  permissions: 0644
  owner: root
  content: |
    [Unit]
    Description=Start lbex
    After=nginx.service

    [Service]
    Environment=HOME=/root
    WorkingDirectory=-/usr/local/bin/lbex
    Restart=always
    ExecStartPre=/usr/bin/curl -o /usr/local/bin/kubectl -LO https://storage.googleapis.com/kubernetes-release/release/\$(curl -s https://storage.googleapis.com/kubernetes-release/release/stable.txt)/bin/linux/amd64/kubectl
    ExecStartPre=/bin/chmod +x /usr/local/bin/kubectl
    ExecStartPre=/bin/bash -c '/usr/bin/curl -so /tmp/lbex.tar.gz -OL "https://github.com/samsung-cnct/lbex/releases/download/\$(/usr/bin/basename "\$(/usr/bin/curl -w "%{url_effective}\\n" -I -L -s -S https://github.com/samsung-cnct/lbex/releases/latest -o /dev/null)")/linux_amd64.tar.gz"'
    ExecStartPre=/bin/mkdir -p /usr/local/bin/lbex
    ExecStartPre=/bin/tar -zxvf /tmp/lbex.tar.gz -C /usr/local/bin/lbex
    ExecStartPre=/bin/chmod +x /usr/local/bin/lbex/lbex
    ExecStartPre=/usr/bin/gcloud config set container/use_application_default_credentials true
    ExecStartPre=/usr/bin/gcloud container clusters get-credentials ${LBEX_CLUSTER_NAME} --zone ${LBEX_CLUSTER_ZONE} --project ${LBEX_PROJECT}
    ExecStartPre=/bin/rm --force /etc/nginx/conf.d/*
    ExecStart=/usr/local/bin/lbex/lbex --kubeconfig /root/.kube/config --health-check --health-port ${LBEX_HEALTH_PORT} --v 2
runcmd:
- systemctl daemon-reload
- systemctl start lbex.service
- systemctl start nginx.service
- curl -o /tmp/install-logging-agent.sh -OL https://dl.google.com/cloudagents/install-logging-agent.sh
- bash /tmp/install-logging-agent.sh
EOF

inf "Creating instance template ${LBEX_BASE_NAME}-instance with external addresses"
gcloud compute instance-templates create \
  ${LBEX_BASE_NAME}-instance \
  --description="${LBEX_BASE_NAME} instance template" \
  --machine-type=n1-standard-1 \
  --metadata-from-file "user-data=${TEMPDIR}/cloud-init" \
  --network="${LBEX_CLUSTER_NETWORK}" \
  --subnet="${LBEX_BASE_NAME}-subnetwork" \
  --region=${LBEX_REGION} \
  --image-project=ubuntu-os-cloud \
  --image-family=ubuntu-1604-lts \
  --scopes=compute-rw,cloud-platform,storage-full,logging-write,monitoring \
  --tags=${LBEX_BASE_NAME} \
  --project=${LBEX_PROJECT}


# create a managed group
inf "Creating managed instance group ${LBEX_BASE_NAME}-instance-group"
gcloud compute instance-groups managed create \
  ${LBEX_BASE_NAME}-instance-group \
  --template="${LBEX_BASE_NAME}-instance" \
  --size=2 \
  --base-instance-name="${LBEX_BASE_NAME}" \
  --description="${LBEX_BASE_NAME} LBEX managed instance group" \
  --region=${LBEX_REGION} \
  --project=${LBEX_PROJECT}

inf "Setting up autoscaling"
gcloud compute instance-groups managed set-autoscaling \
  ${LBEX_BASE_NAME}-instance-group \
  --max-num-replicas=${LBEX_MAX_AUTOSCALE} \
  --description="${LBEX_BASE_NAME}-instance-group autoscaler" \
  --min-num-replicas=2 \
  --scale-based-on-cpu \
  --target-cpu-utilization=0.7 \
  --region=${LBEX_REGION} \
  --project=${LBEX_PROJECT}

inf "Creating healthcheck ${LBEX_BASE_NAME}-healthcheck"
gcloud compute http-health-checks create \
  ${LBEX_BASE_NAME}-healthcheck \
  --description="${LBEX_BASE_NAME} health checker" \
  --check-interval=5s \
  --healthy-threshold=2 \
  --port=${LBEX_HEALTH_PORT} \
  --timeout=5s \
  --unhealthy-threshold=2 \
  --project=${LBEX_PROJECT}

inf "Setting up autohealing"
gcloud beta compute instance-groups managed set-autohealing \
  ${LBEX_BASE_NAME}-instance-group \
  --initial-delay=180 \
  --http-health-check=${LBEX_BASE_NAME}-healthcheck \
  --region=${LBEX_REGION} \
  --project=${LBEX_PROJECT}

