FROM golang:1.12.9-alpine3.9 AS build-env
WORKDIR /usr/local/go/src/github.com/dotmesh-io/github-issue-janitor
RUN apk update && apk upgrade && \
    apk add --no-cache bash git openssh
COPY . /usr/local/go/src/github.com/dotmesh-io/github-issue-janitor
RUN cd cmd/janitor && go install -ldflags="-w -s"

# Can't be "FROM scratch" as we need a whole load of x.509 cert infrastructure I don't wanna build by hand.
FROM ubuntu:xenial
RUN apt-get update && apt-get install -y ca-certificates
COPY --from=build-env /usr/local/go/bin/janitor /bin/janitor
CMD ["/janitor"]