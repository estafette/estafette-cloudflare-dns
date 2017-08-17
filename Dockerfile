FROM scratch

MAINTAINER estafette.io

COPY ca-certificates.crt /etc/ssl/certs/
COPY estafette-cloudflare-dns /

ENTRYPOINT ["/estafette-cloudflare-dns"]
