# create archive of golang dependancies
FROM golang:alpine as builder

RUN apk add --no-cache make gcc git musl-dev
COPY Makefile ~/Makefile
RUN make godeps
RUN tar -zcf /tmp/go.tgz -C /go/ ./

# create a base image with necessary tooling and dependancies cache
FROM golang:alpine
LABEL maintainer="cblomart@gmail.com"

RUN apk add --no-cache make gcc git upx docker musl-dev
RUN mkdir -p /var/cache/drone
COPY --from=builder /tmp/go.tgz /var/cache/drone/go.tgz
