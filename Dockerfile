# Can't be "FROM scratch" as we need a whole load of x.509 cert infrastructure I don't wanna build by hand.
FROM ubuntu:xenial
RUN apt-get update && apt-get install -y ca-certificates
COPY target/janitor /
CMD ["/janitor"]