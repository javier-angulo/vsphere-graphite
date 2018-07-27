FROM golang:alpine
LABEL maintainer="cblomart@gmail.com"

RUN apk add --no-cache make gcc git upx docker musl-dev

COPY Makefile /tmp/Makefile

RUN cd /tmp/; make deps