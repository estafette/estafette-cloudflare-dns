FROM scratch

COPY ca-certificates.crt /etc/ssl/certs/
COPY estafette-cloudflare-dns /

CMD ["/estafette-cloudflare-dns"]