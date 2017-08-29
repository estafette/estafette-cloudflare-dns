FROM scratch

LABEL maintainer="estafette.io" \
      description="The estafette-cloudflare-dns component is a Kubernetes controller that sets dns records in Cloudflare for annotated Kubernetes services and ingresses"

COPY ca-certificates.crt /etc/ssl/certs/
COPY estafette-cloudflare-dns /

ENTRYPOINT ["/estafette-cloudflare-dns"]
