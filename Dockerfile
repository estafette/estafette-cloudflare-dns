FROM scratch

COPY ca-certificates.crt /etc/ssl/certs/
COPY main /

CMD ["/main"]