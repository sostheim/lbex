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
  inf "-n|--name          - base name for all lbex resource. Required."
  inf "-p|--project       - Google project id. Required."
  inf "-c|--cidr          - CIDR range for LBEX instance IP addresses, big enough for at least 'num-autoscale' IPs. Default is 10.128.0.0/28."
  inf "-r|--region        - GCE region name. Default is us-central1."
  inf "-m|--cluster-name  - Target GKE cluster name. Required for gce-up.sh."
  inf "-n|--num-autoscale - Maximum number of autoscaled LBEX instances. Default is 10."
  inf "-z|--cluster-zone  - Target GKE cluster primary zone. Default is us-central1-a."
  inf "-p|--ports         - Comma-delimited ports and port ranges for LBEX. Docker port-publishing format. I.e. '80,443,3456/udp,7000-8000' Default is '80,443'"
  inf "-e|--health-port   - LBEX healthcheck port. Default is 7331."
  inf "-h|--help          - Show this help.\n"

  inf "For example:"
  inf "gce-up.sh --name mylbex --project k8s-work --cidr 10.128.0.0/28 --region us-central1 --cluster-name my-k8s-cluster --num-autoscale 10 --cluster-zone us-central1-a --ports 80,443,9827/udp --health-port 1337"
  inf "or"
  inf "gce-down.sh --name mylbex"
}

while [[ $# -gt 0 ]]
do
key="$1"
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
  -m|--cluster-name)
  LBEX_CLUSTER_NAME="$2"
  shift
  ;;
  -n|--num-autoscale)
  LBEX_MAX_AUTOSCALE="$2"
  shift
  ;;
  -z|--cluster-zone)
  LBEX_CLUSTER_ZONE="$2"
  shift
  ;;
  -p|--ports)
  LBEX_PORTS="$2"
  shift
  ;;
  -e|--health-port)
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



