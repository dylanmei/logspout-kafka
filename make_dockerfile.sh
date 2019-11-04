#!bin/bash

cat > ./Dockerfile.example <<DOCKERFILE
FROM gliderlabs/logspout:master

ENV KAFKA_COMPRESSION_CODEC snappy
DOCKERFILE

cat > ./modules.go <<MODULES
package main
import (
  _ "github.com/gliderlabs/logspout/httpstream"
  _ "github.com/gliderlabs/logspout/routesapi"
  _ "github.com/akingston2/logspout-kafka"
)
MODULES

docker build -t gettyimages/example-logspout -f Dockerfile.example .