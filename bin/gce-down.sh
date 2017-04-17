#!/bin/bash -
#title           :gce-down.sh
#description     :bring down an LBEX group
#author          :Samsung SDSRA
#==============================================================================
set -o errexit
set -o nounset
set -o pipefail

# pull in utils
my_dir=$(dirname "${BASH_SOURCE}")
source "${my_dir}/utils.sh"

if [ -z "${LBEX_REGION+x}" ]; then
  LBEX_REGION='us-central1'
  inf "Using '${LBEX_REGION}' as region"
fi

# delete the managed group
gcloud compute instance-groups managed delete \
  ${LBEX_BASE_NAME}-instance-group \
  --region=${LBEX_REGION} \
  --project=${LBEX_PROJECT} --quiet || true

# delete the healthcheck
gcloud compute http-health-checks delete \
  ${LBEX_BASE_NAME}-healthcheck \
  --project=${LBEX_PROJECT} --quiet || true

# delete the instance template
gcloud compute instance-templates delete \
  ${LBEX_BASE_NAME}-instance \
  --project=${LBEX_PROJECT} --quiet || true

# delete all-firewall rule
gcloud compute firewall-rules delete \
  ${LBEX_BASE_NAME}-all-traffic \
  --project=${LBEX_PROJECT} --quiet || true

# delete the subnet
gcloud compute networks subnets delete \
  ${LBEX_BASE_NAME}-subnetwork \
  --region=${LBEX_REGION} \
  --project=${LBEX_PROJECT} --quiet || true
