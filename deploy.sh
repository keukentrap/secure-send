#!/bin/bash

set -e

HOST=$1
IMAGE=$2
CONTAINER_NAME=$3

ssh -o StrictHostKeyChecking=no -p 22000 $HOST "
mkdir -p /tmp/cache

docker stop $CONTAINER_NAME
docker rm $CONTAINER_NAME
docker pull $IMAGE
docker run --name $CONTAINER_NAME -v /tmp/cache:/app/cache -p 9999:9999 -d $IMAGE
docker system prune --all --force
"
