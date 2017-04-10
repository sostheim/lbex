#!/bin/bash

function show_help {
  echo -e "Usage: \n"
}

case $key in
  -n|--name)
  LBEX_BASE_NAME="$2"
  shift
  ;;
  -p|--project)
  LBEX_PROJECT="$2"
  shift
  ;;
  -c|--cidr)
  LBEX_CIDR="$2"
  shift
  ;;
  -r|--region)
  LBEX_REGION="$2"
  shift
  ;;
  -z|--zone)
  LBEX_ZONE="$2"
  shift
  ;;
  -m|--cluster-name)
  LBEX_CLUSTER_NAME="$2"
  shift
  ;;
  -m|--cluster-zone)
  LBEX_CLUSTER_ZONE="$2"
  shift
  ;;
  -h|--help)
  LBEX_HELP=true
  ;;
  *)
  LBEX_HELP=true
  ;;
esac
shift # past argument or value
done

if [ -z "${LBEX_BASE_NAME+x}" ]; then
  LBEX_BASE_NAME='sds-lbex'
  echo -e "Using '${LBEX_BASE_NAME}' as base name for all resources"
fi

if [ -n "${LBEX_PROJECT+x}" ]; then
  show_help
  exit 0
fi

if [ -z "${LBEX_CIDR+x}" ]; then
  LBEX_CIDR='10.128.0.0/28'
  echo -e "Using '${LBEX_CIDR}' as CIDR"
fi

if [ -z "${LBEX_REGION+x}" ]; then
  LBEX_REGION='us-central1'
  echo -e "Using '${LBEX_REGION}' as region"
fi

if [ -z "${LBEX_ZONE+x}" ]; then
  LBEX_ZONE='us-central1-a'
  echo -e "Using '${LBEX_ZONE}' as zone"
fi

if [ -z "${LBEX_CLUSTER_NAME+x}" ]; then
  show_help
  exit 0
fi

if [ -z "${LBEX_CLUSTER_ZONE+x}" ]; then
  LBEX_CLUSTER_ZONE="${LBEX_ZONE}"
  echo -e "Using '${LBEX_CLUSTER_ZONE}' as gke cluster zone"
fi

if [ -n "${LBEX_HELP+x}" ]; then
  show_help
  exit 0
fi

# create a custom network and subnet
gcloud compute networks create \
  ${LBEX_BASE_NAME}-network \
  --description="Network for ${LBEX_BASE_NAME} LBEX instances" \
  --mode=custom \
  --project=${LBEX_PROJECT}

gcloud compute networks subnets create \
  {LBEX_BASE_NAME}-subnetwork \
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

# create 'templated' cloud init for the instance template
TEMPDIR=$(mktemp -d "${TMPDIR:-/tmp/}$(basename 0).XXXXXXXXXXXX")
cat "${TEMPDIR}/cloud-init" > /dev/null <<EOF
#cloud-config
write_files:
- path: /etc/systemd/system/lbex.service
  permissions: 0644
  owner: root
  content: |
    [Unit]
    Description=Start lbex container

    [Service]
    ExecStartPre=/bin/mkdir -p /var/kubeconfig
    ExecStartPre=/usr/bin/toolbox /google-cloud-sdk/bin/gcloud components install kubectl -q
    ExecStartPre=/usr/bin/toolbox --bind=/var/kubeconfig:/root/.kube /google-cloud-sdk/bin/gcloud container clusters get-credentials ${LBEX_CLUSTER_NAME} --zone ${LBEX_CLUSTER_ZONE} --project ${LBEX_PROJECT}
    ExecStart=/usr/bin/docker run --rm --name=lbex --volume /var/kubeconfig:/kubeconfig quay.io/samsung_cnct/lbex:latest --kubeconfig /kubeconfig/config
    ExecStop=/usr/bin/docker stop lbex
    ExecStopPost=/usr/bin/docker rm lbex

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
  --image-project=cos-cloud \
  --image-family=cos-stable \
  --scopes=compute-rw,cloud-platform,storage-full,logging-write,monitoring \
  --project=${LBEX_PROJECT}

// create a managed group
gcloud compute instance-groups managed create \
  lbex-instance-group \
  --template="${LBEX_BASE_NAME}-instance" \
  --size=2 \
  --base-instance-name="${LBEX_BASE_NAME}" \
  --description="${LBEX_BASE_NAME} LBEX managed instance group" \
  --region=${LBEX_REGION} \
  --zone=${LBEX_ZONE} \
  --project=${LBEX_PROJECT}





