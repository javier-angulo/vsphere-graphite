# create archive of golang dependancies
FROM golang:1.10 as builder

COPY Makefile Makefile
RUN make godeps
RUN tar -zcf /tmp/go.tgz -C /go/ ./

FROM docker AS docker

# create a base image with necessary tooling and dependancies cache
FROM golang:1.10
LABEL maintainer="cblomart@gmail.com"

RUN apt-get update && apt-get install -y \
    upx-ucl\
    musl\
    musl-tools &&\
    apt-get clean &&\
    rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*

#RUN curl -SL https://download.docker.com/linux/static/stable/x86_64/docker-18.06.0-ce.tgz | tar -zxC /tmp &&\
#    cp /tmp/docker/docker /usr/local/bin/docker &&\
#    rm -rf /tmp/docker

COPY --from=docker /usr/local/bin/docker /usr/local/bin/docker

RUN mkdir -p /var/cache/drone
COPY --from=builder /tmp/go.tgz /var/cache/drone/go.tgz
