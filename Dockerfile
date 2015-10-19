FROM scratch
MAINTAINER Alfonso Acosta <alfonso.acosta@gmail.com>
WORKDIR /home/arriba
ADD ca-certificates.crt /etc/ssl/certs/
ADD ./arriba /home/arriba/
ENTRYPOINT ["/home/arriba/arriba"]
