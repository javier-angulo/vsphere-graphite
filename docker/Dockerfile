FROM scratch
MAINTAINER cblomart@gmail.com

COPY ./vsphere-graphite /vsphere-graphite
COPY ./vsphere-graphite.json /etc/vsphere-graphite.json
# Backends my require TLS:
# COPY ./ca-certificates.crt /etc/ssl/certs/

VOLUME [ "/etc" ]

ENTRYPOINT [ "/vsphere-graphite" ] 