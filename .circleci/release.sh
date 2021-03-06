#!/bin/bash -e

if [[ $CIRCLE_TAG == v* ]] ;
then
  VERSION=$CIRCLE_TAG
else
  VERSION=experimental
fi

$(aws ecr get-login --no-include-email --region us-west-2)
docker login -u $DOCKER_HUB_LOGIN -p $DOCKER_HUB_PASSWORD

docker pull $NODE_DOCKER_IMAGE:$(./docker/hash.sh)

docker tag $NODE_DOCKER_IMAGE:$(./docker/hash.sh) orbsnetwork/node:$VERSION
docker push orbsnetwork/node:$VERSION

docker tag $NODE_DOCKER_IMAGE:$(./docker/hash.sh) orbsnetwork/node:$(./docker/hash.sh)
docker push orbsnetwork/node:$(./docker/hash.sh)

docker pull $GAMMA_DOCKER_IMAGE:$(./docker/hash.sh)
docker tag $GAMMA_DOCKER_IMAGE:$(./docker/hash.sh) orbsnetwork/gamma:$VERSION
docker push orbsnetwork/gamma:$VERSION

docker pull $SIGNER_DOCKER_IMAGE:$(./docker/hash.sh)
docker tag $SIGNER_DOCKER_IMAGE:$(./docker/hash.sh) orbsnetwork/signer:$VERSION
docker push orbsnetwork/signer:$VERSION
