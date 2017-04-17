#!/bin/bash -
#title           :utils.sh
#description     :utils
#author          :Samsung SDSRA
#==============================================================================

my_dir=$(dirname "${BASH_SOURCE}")

function warn {
  echo -e "\033[1;33mWARNING: $1\033[0m"
}

function error {
  echo -e "\033[0;31mERROR: $1\033[0m"
}

function inf {
  echo -e "\033[0;32m$1\033[0m"
}

function show_help {
  inf "Usage: \n"
  inf "gce-up.sh or gce-down.sh [flags] \n"
  inf "Flags are:"
  inf "-c|--cidr            - CIDR range for LBEX instance IP addresses, big enough for at least 'num-autoscale' IPs. Should not clash with GKE cluster ip space. Default is 10.150.0.0/28."
  inf "-h|--help            - Show this help.\n"
  inf "-i|--project         - Google project id. Required."
  inf "-m|--cluster-name    - Target GKE cluster name. Required for gce-up.sh."
  inf "-n|--name            - base name for all lbex resource. Required."
  inf "-p|--health-port     - LBEX healthcheck port. Default is 7331."
  inf "-r|--region          - GCE region name. Default is us-central1."
  inf "-s|--num-autoscale   - Maximum number of autoscaled LBEX instances. Default is 10."
  inf "-t|--cluster-network - GCE network name of the GKE cluster. Must be a custom type network. Required"
  inf "-z|--cluster-zone    - Target GKE cluster primary zone. Default is us-central1-a."

  inf "For example:"
  inf "gce-up.sh --name mylbex --project k8s-work --cidr 10.128.0.0/28 --region us-central1 --cluster-name my-k8s-cluster --num-autoscale 10 --cluster-zone us-central1-a --cluster-network my-cluster-network --health-port 1337"
  inf "or"
  inf "gce-down.sh --name mylbex --project k8s-work"
}

while [[ $# -gt 0 ]]
do
key="$1"
case $key in
  -n|--name)
  LBEX_BASE_NAME="$2"
  shift
  ;;
  -i|--project)
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
  -m|--cluster-name)
  LBEX_CLUSTER_NAME="$2"
  shift
  ;;
  -t|--cluster-network)
  LBEX_CLUSTER_NETWORK="$2"
  shift
  ;;
  -s|--num-autoscale)
  LBEX_MAX_AUTOSCALE="$2"
  shift
  ;;
  -z|--cluster-zone)
  LBEX_CLUSTER_ZONE="$2"
  shift
  ;;
  -p|--health-port)
  LBEX_HEALTH_PORT="$2"
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

if [ -n "${LBEX_HELP+x}" ]; then
  show_help
  exit 0
fi

if [ -z "${LBEX_PROJECT+x}" ]; then
  show_help
  exit 1
fi

if [ -z "${LBEX_BASE_NAME+x}" ]; then
  show_help
  exit 1
fi



