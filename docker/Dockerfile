# STEP 1 build executable binary
FROM golang:alpine as builder
# Create nonroot user
RUN adduser -D -g '' vsphere-graphite-user

# STEP 2 build a small image from scratch
FROM scratch
MAINTAINER cblomart@gmail.com
COPY --from=builder /etc/passwd /etc/passwd
COPY ./vsphere-graphite /vsphere-graphite
COPY ./vsphere-graphite.json /etc/vsphere-graphite.json
# Backends might require TLS:
# COPY ./ca-certificates.crt /etc/ssl/certs/
USER vsphere-graphite-user
VOLUME [ "/etc" ]

ENTRYPOINT [ "/vsphere-graphite" ] 