#!/usr/bin/env bash

kubectl config set current-context gke_craigskelton-com_us-central1-a_cluster-1

mainmenu () {
  echo "Press 1 to build"
  echo "Press 2 to deploy"
  echo "Press 3/enter to run away screaming"
  echo "Press 9 to nuke test. Make sure you're shure"
  read  -n 1 -p "Input Selection:" mainmenuinput
  echo
  if [ "$mainmenuinput" = "1" ]; then
            build
        elif [ "$mainmenuinput" = "2" ]; then
            deploy
        else
            exit
        fi
}

build () {
    echo 'cross compiling'
    ./crosscompile.sh
}

deploy () {
    echo 'deploy to kubernetes'
    DOCKER_TAG=`date +"%m-%d-%Y-%H-%M-%S"`
    docker build -t gcr.io/craigskelton-com/cluebatbot:$DOCKER_TAG .
    docker push gcr.io/craigskelton-com/cluebatbot:$DOCKER_TAG
    echo "Docker tag $DOCKER_TAG pushed to gcr.io"
    kubectl -n cluebatbot apply -f k8s/deployment.yaml
    kubectl -n cluebatbot set image deployment/cluebatbot cluebatbot=gcr.io/craigskelton-com/cluebatbot:$DOCKER_TAG
}

while true; do
    mainmenu
done
