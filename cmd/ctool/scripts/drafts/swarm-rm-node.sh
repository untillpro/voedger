#!/usr/bin/env bash
#
# Copyright (c) 2023 Sigma-Soft, Ltd.
# @author Aleksei Ponomarev
#
# Delete node from Swarm cluster. 
#    - find node id by ip address
#    - Since manager cannot be removed from swarm, 
#        - first, manager demote to worker 
#        - then sequentally removed from swarm

set -euo pipefail

set +x

if [ "$#" -lt 2 ]; then
  echo "Usage: $0 <swarm manager> <removing node> <...>" >&2
  exit 1
fi

SSH_USER=$LOGNAME
SSH_OPTIONS='-o UserKnownHostsFile=/dev/null -o StrictHostKeyChecking=no -o LogLevel=ERROR'
MANAGER=$1

JOIN_TOKEN=$(cat ./manager.token)

shift 
# Add remaining nodes as managers and workers
while [ $# -gt 0 ]; do

ip=$(getent hosts "$1" | awk '{print $1}')

# Get the ID of the node with the specified IP address
node_id=$(ssh $SSH_OPTIONS $SSH_USER@$MANAGER "docker node ls --format '{{.ID}}' | while read id; do docker node inspect --format '{{.Status.Addr}} {{.ID}}' \$id; done | grep $ip | awk '{print \$2}'")
  if [[ -n "$node_id" ]]; then
    ssh $SSH_OPTIONS $SSH_USER@$MANAGER "docker node demote $node_id && docker node rm $node_id"
  fi

shift

done

set +x
