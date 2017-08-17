FROM scratch

MAINTAINER estafette.io

RUN addgroup estafette && adduser -g estafette estafette 

COPY ca-certificates.crt /etc/ssl/certs/
COPY estafette-cloudflare-dns /

USER estafette

ENTRYPOINT ["/estafette-cloudflare-dns"]
