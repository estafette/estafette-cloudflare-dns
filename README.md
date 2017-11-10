# estafette-cloudflare-dns

This small Kubernetes application configures dns and proxy settings in Cloudflare for any public service with the correct annotations

[![License](https://img.shields.io/github/license/estafette/estafette-cloudflare-dns.svg)](https://github.com/estafette/estafette-cloudflare-dns/blob/master/LICENSE)

## Why?

In order not to have to set dns records manually or from deployment scripts this application decouples that responsibility and moves it into the Kubernetes cluster itself.

## Usage

Deploy with Helm:

```
brew install kubernetes-helm
helm init --history-max 25 --upgrade
lint helm chart with helm lint chart/estafette-cloudflare-dns
chart helm package chart/estafette-cloudflare-dns --version 1.0.103
helm upgrade estafette-cloudflare-dns estafette-cloudflare-dns-1.0.103.tgz --namespace estafette --install --dry-run --debug --set cloudflareApiEmail=*** --set cloudflareApiKey=*** --set rbac.create=true
```

Or deploy without Helm:

```
curl https://raw.githubusercontent.com/estafette/estafette-cloudflare-dns/master/kubernetes.yaml -o kubernetes.yaml

export NAMESPACE=estafette
export APP_NAME=estafette-cloudflare-dns
export TEAM_NAME=tooling
export VERSION=1.0.103
export GO_PIPELINE_LABEL=1.0.103
export CF_API_EMAIL=***
export CF_API_KEY=***
export CPU_REQUEST=10m
export MEMORY_REQUEST=15Mi
export CPU_LIMIT=50m
export MEMORY_LIMIT=128Mi

cat kubernetes.yaml | envsubst | kubectl apply -n ${NAMESPACE} -f -
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