#!/usr/bin/env bash
#
# Copyright (c) 2023 Sigma-Soft, Ltd.
# @author Aleksei Ponomarev
# 
# Prepare scylla node: create directory for scylla data files,
# and copy scylla config to host

set -euo pipefail

set -x

if [[ $# -ne 5 ]]; then
  echo "Usage: $0 <SE node1> <SE node2> <Scylla node1> <Scylla node2> <Scylla node3>" 
  exit 1
fi


se1=$1
se2=$2
scylla1=$3
scylla2=$4
scylla3=$5

SSH_USER=$LOGNAME
SSH_OPTIONS='-o UserKnownHostsFile=/dev/null -o StrictHostKeyChecking=no -o LogLevel=ERROR'

count=0

while [ $# -gt 0 ] && [ $count -lt 2 ]; do
  echo "Processing: $1"
  ssh $SSH_OPTIONS $SSH_USER@$1 "sudo mkdir -p /prometheus && mkdir -p ~/prometheus"
  ssh $SSH_OPTIONS $SSH_USER@$1 "sudo mkdir -p /alertmanager && mkdir -p ~/alertmanager"
  ssh $SSH_OPTIONS $SSH_USER@$1 "mkdir -p ~/grafana/provisioning/dashboards"
  ssh $SSH_OPTIONS $SSH_USER@$1 "mkdir -p ~/grafana/provisioning/datasources"

   cat ./grafana/provisioning/dashboards/swarmprom-nodes-dash.json | \
      ssh $SSH_OPTIONS $SSH_USER@$1 'cat > ~/grafana/provisioning/dashboards/swarmprom-nodes-dash.json'
   cat ./grafana/provisioning/dashboards/swarmprom-prometheus-dash.json | \
      ssh $SSH_OPTIONS $SSH_USER@$1 'cat > ~/grafana/provisioning/dashboards/swarmprom-prometheus-dash.json'
   cat ./grafana/provisioning/dashboards/swarmprom-services-dash.json | \
      ssh $SSH_OPTIONS $SSH_USER@$1 'cat > ~/grafana/provisioning/dashboards/swarmprom-services-dash.json'
   cat ./grafana/provisioning/dashboards/swarmprom_dashboards.yml | \
      ssh $SSH_OPTIONS $SSH_USER@$1 'cat > ~/grafana/provisioning/dashboards/swarmprom_dashboards.yml'

  cat ./grafana/provisioning/datasources/datasource.yml | \
      sed "s/{se}/$1/g" \
      | ssh $SSH_OPTIONS $SSH_USER@$1 'cat > ~/grafana/provisioning/datasources/datasource.yml'

  cat ./grafana/grafana.ini | ssh $SSH_OPTIONS $SSH_USER@$1 'cat > ~/grafana/grafana.ini'


  cat ./prometheus/prometheus.yml | \
      sed "s/{scylla1}/$scylla1/g; s/{scylla2}/$scylla2/g; s/{scylla3}/$scylla3/g; s/{se1}/$se1/g; s/{se2}/$se2/g; s/{mon_label}/mon$((count+1))/g" \
      | ssh $SSH_OPTIONS $SSH_USER@$1 'cat > ~/prometheus/prometheus.yml'

  cat ./prometheus/alert.rules | ssh $SSH_OPTIONS $SSH_USER@$1 'cat > ~/prometheus/alert.rules'
  cat ./alertmanager/config.yml | ssh $SSH_OPTIONS $SSH_USER@$1 'cat > ~/alertmanager/config.yml'

  ssh $SSH_OPTIONS $SSH_USER@$1 "sudo chown -R 65534:65534 /prometheus"
  ssh $SSH_OPTIONS $SSH_USER@$1 "sudo chown -R 65534:65534 /alertmanager"
  ssh $SSH_OPTIONS $SSH_USER@$1 "sudo chown -R 472:472 /var/lib/grafana"

  count=$((count+1))

  shift
done

set +x
