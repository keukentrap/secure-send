#!/bin/bash

HOST=$1
ID=$2

ssh -i $ID -p 22000 $HOST "
IMAGE='ghcr.io/keukentrap/secure-send:main'
CONTAINER_NAME='secure-send'

mkdir -p ~/deploy/secure-send
cd ~/deploy/secure-send

docker stop $CONTAINER_NAME
docker rm $CONTAINER_NAME
docker pull $IMAGE
docker run --name $CONTAINER_NAME -p 9999:9999 -d $IMAGE

"
