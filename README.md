# estafette-cloudflare-dns

This small Kubernetes application configures dns and proxy settings in Cloudflare for any public service with the correct annotations

[![License](https://img.shields.io/github/license/estafette/estafette-cloudflare-dns.svg)](https://github.com/estafette/estafette-cloudflare-dns/blob/master/LICENSE)

## Why?

In order not to have to set dns records manually or from deployment scripts this application decouples that responsibility and moves it into the Kubernetes cluster itself.

## Installation

Prepare using Helm:

```
brew install kubernetes-helm
kubectl -n kube-system create serviceaccount tiller
kubectl create clusterrolebinding tiller --clusterrole=cluster-admin --serviceaccount=kube-system:tiller
helm init --service-account tiller --wait
```

Then install or upgrade with Helm:

```
helm repo add estafette https://helm.estafette.io
helm upgrade --install estafette-cloudflare-dns --namespace estafette estafette/estafette-cloudflare-dns
```

## Usage

Once it's running put the following annotations on a service of type LoadBalancer and deploy. The `estafette-cloudflare-dns` controller will watch changes to services and process those. Once approximately every 300 seconds it also scans all services as a safety net in case an event has been missed.

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