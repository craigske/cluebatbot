#!/usr/bin/env bash

# ./dep-windows-amd64 ensure -update
dep ensure -update

env CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -a -installsuffix nocgo -o bin/cluebatbot.darwin .
echo "Built cluebatbot for mac"
env CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -a -installsuffix nocgo -o bin/cluebatbot.exe .
echo "Built cluebatbot for windows"
env CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix nocgo -o bin/cluebatbot.linux.amd64 .
echo "Built cluebatbot for linux"
env CGO_ENABLED=0 GOOS=linux GOARCH=arm go build -a -installsuffix nocgo -o bin/cluebatbot.pi .
echo "Built cluebatbot for pi"

chmod 755 bin/cluebatbot.*