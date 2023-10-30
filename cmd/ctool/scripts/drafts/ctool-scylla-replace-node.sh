#!/usr/bin/env bash
#
# Copyright (c) 2023 Sigma-Soft, Ltd.
# @author Aleksei Ponomarev
#
# Replace scylla node in cluster 
#   - Install docker on new hardware
#   - Add new node to swarm
#   - Prepare node to scylla service
#   - Prepare node to bootstrap
#   - Remove lost node from swarm cluster
#   - Prepare updated scylla stack
#   - deploy stack
#   - add label to new node, to place service
#   - add node to back seed list and restart service
#   - rolling restart db cluster

set -euo pipefail

set -x

if [[ $# -ne 3 ]]; then
  echo "Usage: $0 <lost node> <new node> <swarm manager> <datacenter>"
  exit 1
fi

SSH_USER=$LOGNAME
SSH_OPTIONS='-o UserKnownHostsFile=~/.ssh/known_hosts -o StrictHostKeyChecking=no -o LogLevel=ERROR'
STACK="DBDockerStack"

MANAGER=$3
DC=$4
REPLACED_NODE_NAME=$(getent hosts "$2" | awk '{print $2}')
ssh-keyscan -H "$REPLACED_NODE_NAME" >> ~/.ssh/known_hosts

wait_for_scylla() {
  local ip_address=$1
  echo "Working with $ip_address"
  local count=0

  while [ $count -lt 100 ]; do
    if [ $(ssh "$SSH_OPTIONS" "$SSH_USER"@"$ip_address" "docker exec \$(docker ps -qf name=scylla) nodetool status | grep -c '^UN\s'") -eq 3 ]; then
      echo "Scylla initialization success"
      return 0
    fi
    echo "Still waiting for Scylla initialization.."
    sleep 5
    count=$((count+1))
  done
  if [ $count -eq 100 ]; then
    echo "Scylla initialization timed out."
    return 1
  fi
}

./docker-install.sh "$2"
./swarm-add-node.sh "$MANAGER" "$2"
./db-node-prepare.sh "$2" "$DC"
./db-bootstrap-prepare.sh "$1" "$2"
./swarm-rm-node.sh "$MANAGER" "$1"

seed_list() {
  local node=$1
  local operation=$2

  service_label=$(./db-stack-update.sh "$node" "$operation" | tail -n 1)
  < ./docker-compose.yml ssh "$SSH_OPTIONS" "$SSH_USER"@"$node" 'cat > ~/docker-compose.yml'
  ssh "$SSH_OPTIONS" "$SSH_USER"@"$node" "docker stack deploy --compose-file ~/docker-compose.yml DBDockerStack"
  ./swarm-set-label.sh "$MANAGER" "$node" "$service_label" "true"
}

echo "Remove dead node from seed list and start db instance on new hardware."
seed_list "$REPLACED_NODE_NAME" remove
  wait_for_scylla "$REPLACED_NODE_NAME"

echo "Bootstrap complete. Cleanup scylla config..."
./db-bootstrap-end.sh "$2"

echo "Add node to seed list and restart."
seed_list "$REPLACED_NODE_NAME" add
  wait_for_scylla "$REPLACED_NODE_NAME"

db_rolling_restart() {
  local compose_file="$1"
  local services=()      

  mapfile -t services < <(yq eval '.services | keys | .[]' "$compose_file")

  for service in "${services[@]}"; do
    echo "Restart service: ${STACK}_${service}"
    docker service update --force "$STACK"_"$service"
    wait_for_scylla "$REPLACED_NODE_NAME"
  done
}
echo "Rolling restart db cluster..."
db_rolling_restart ./docker-compose.yml

set +x
