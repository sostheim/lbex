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

# create lbex subnet
gcloud compute networks subnets create \
  ${LBEX_BASE_NAME}-subnetwork \
  --description="Sub-network for ${LBEX_BASE_NAME}-LBEX instances" \
  --network="${LBEX_CLUSTER_NETWORK}" \
  --range=${LBEX_CIDR} \
  --region=${LBEX_REGION} \
  --project=${LBEX_PROJECT}

# add firewall rules
gcloud compute firewall-rules create \
  ${LBEX_BASE_NAME}-internal \
  --network="${LBEX_CLUSTER_NETWORK}" \
  --allow tcp,udp,icmp \
  --source-ranges=0.0.0.0/0 \
  --target-tags=${LBEX_BASE_NAME} \
  --project=${LBEX_PROJECT}

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
    Environment=USER=root
    Restart=always
    ExecStartPre=/usr/bin/curl -o /usr/local/bin/kubectl -LO https://storage.googleapis.com/kubernetes-release/release/\$(curl -s https://storage.googleapis.com/kubernetes-release/release/stable.txt)/bin/linux/amd64/kubectl
    ExecStartPre=/bin/chmod +x /usr/local/bin/kubectl
    ExecStartPre=/usr/bin/curl -s -o /usr/local/bin/jq -LO https://github.com/stedolan/jq/releases/download/jq-1.5/jq-linux64
    ExecStartPre=/bin/chmod +x /usr/local/bin/jq
    ExecStartPre=/bin/bash -c '/usr/bin/curl -so /tmp/lbex.tar.gz -OL "\$(/usr/local/bin/jq -r ".assets[] | select(.name | test(\"linux_amd64.tar.gz\")) | .browser_download_url" < <( /usr/bin/curl -s "https://api.github.com/repos/samsung-cnct/lbex/releases/latest" ))"'
    ExecStartPre=/bin/tar -zxvf /tmp/lbex.tar.gz -C /usr/local/bin
    ExecStartPre=/bin/chmod +x /usr/local/bin/lbex
    ExecStartPre=/usr/bin/gcloud container clusters get-credentials ${LBEX_CLUSTER_NAME} --zone ${LBEX_CLUSTER_ZONE} --project ${LBEX_PROJECT}
    ExecStart=/usr/local/bin/lbex --kubeconfig /root/.kube/config --health-check --health-port ${LBEX_HEALTH_PORT}
runcmd:
- systemctl daemon-reload
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
  --image-project=debian-cloud \
  --image-family=debian-8 \
  --scopes=compute-rw,cloud-platform,storage-full,logging-write,monitoring \
  --tags=${LBEX_BASE_NAME} \
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

