#!/bin/sh

set -ex

docker build -f Dockerfile.build -t janitor-builder:latest .

docker rm -f dotmesh-janitor-builder || true

docker run \
       --name dotmesh-janitor-builder \
       -e GOPATH=/go \
       -e CGO_ENABLED=0 \
       -w /go/src/github.com/dotmesh-io/github-issue-janitor/cmd/janitor \
       janitor-builder:latest \
       go build -a -ldflags "-extldflags \"-static\" " -o /target/janitor .

mkdir -p target

docker cp dotmesh-janitor-builder:/target/janitor target/
docker rm -f dotmesh-janitor-builder

docker build -f Dockerfile -t "github-issue-janitor:latest" .
