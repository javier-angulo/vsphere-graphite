FROM scratch
MAINTAINER cblomart@gmail.com

COPY ./vsphere-graphite /vsphere-graphite
COPY ./vsphere-graphite.json /etc/vsphere-graphite.json

VOLUME [ "/etc" ]

ENTRYPOINT [ "/vsphere-graphite" ] 