# create archive of golang dependancies
FROM golang:1.10

COPY Makefile Makefile
RUN make godeps
RUN tar -zcf /tmp/go.tgz -C /go/ ./

# create a base image with necessary tooling and dependancies cache
FROM golang:1.10
LABEL maintainer="cblomart@gmail.com"

RUN apt-get update && apt-get install -y \
    upx-ucl\
    musl\
    musl-tools &&\
    apt-get clean &&\
    rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*
RUN mkdir -p /var/cache/drone
COPY --from=builder /tmp/go.tgz /var/cache/drone/go.tgz
