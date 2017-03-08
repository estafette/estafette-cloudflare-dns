# estafette-cloudflare-dns

This small Kubernetes application configures dns and proxy settings in Cloudflare for any public service with the correct annotation

[![License](https://img.shields.io/github/license/Travix-International/Hystrix.Dotnet.svg)](https://github.com/Travix-International/Hystrix.Dotnet/blob/master/LICENSE)

## Why?

In order not to have to set dns records manually or from deployment scripts this application decouples that responsibility and moves it into the Kubernetes cluster itself.

## Usage

First deploy this application to your Kubernetes cluster using the following manifest. Make sure to pass an email address and cloudflare api key.

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: estafette
---
apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  name: estafette-cloudflare-dns
  namespace: estafette
  labels:
    app: estafette-cloudflare-dns
spec:
  replicas: 1
  strategy:
    type: Recreate
  selector:
    matchLabels:
      app: estafette-cloudflare-dns
  template:
    metadata:
      labels:
        app: estafette-cloudflare-dns
    spec:
      containers:
      - name: estafette-cloudflare-dns
        image: estafette/estafette-cloudflare-dns:latest
        env:
        - name: "CF_API_EMAIL"
          value: "myemail@mydomain.com"
        - name: "CF_API_KEY"
          value: "****"
        resources:
          requests:
            cpu: 10m
            memory: 16Mi
          limits:
            cpu: 50m
            memory: 128Mi
        livenessProbe:
          httpGet:
            path: /metrics
            port: 9101
          initialDelaySeconds: 30
          timeoutSeconds: 1
```

Once it's running put the following annotations on a service of type LoadBalancer and deploy. The estafette-cloudflare-dns application will watch changes to services and process those. Once approximately every 300 seconds it also scans all services as a safety net.

```yaml

apiVersion: v1
kind: Service
metadata:
  name: myapplication
  namespace: mynamespace
  labels:
    app: myapplication
  annotations:
    estafette.io/cloudflare-dns: "true"
    estafette.io/cloudflare-proxy: "true"
    estafette.io/cloudflare-use-origin-record: "false"
    estafette.io/cloudflare-origin-record-hostname: ""
    estafette.io/cloudflare-hostnames: "mynamespace.mydomain.com"
spec:
  type: LoadBalancer
  ports:
  - name: http
    port: 80
    targetPort: http
    protocol: TCP
  selector:
    app: myapplication
```