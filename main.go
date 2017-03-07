package main

import (
	"context"
	"fmt"
	"log"

	"github.com/ericchiang/k8s"
)

func main() {
	client, err := k8s.NewInClusterClient()
	if err != nil {
		log.Fatal(err)
	}

	services, err := client.CoreV1().ListServices(context.Background(), client.Namespace)
	if err != nil {
		log.Fatal(err)
	}

	for _, service := range services.Items {

		if &service != nil && &service.Metadata != nil && &service.Metadata.Annotations != nil {

			fmt.Printf("svc name=% annotations=%", service.Metadata.Name, service.Metadata.Annotations)

			for key, value := range service.Metadata.Annotations {

				// travix.io/kube-cloudflare-dns: "${CLOUDFLARE_CREATE_DNS_RECORD}"
				// travix.io/kube-cloudflare-proxy: "${CLOUDFLARE_ENABLE_PROXY}"
				// travix.io/kube-cloudflare-use-origin-record: "${CLOUDFLARE_USE_ORIGIN_AND_CNAME_DNS_RECORDS}"
				// travix.io/kube-cloudflare-hostnames: "${HOSTNAMES}"

				if key == "travix.io/kube-cloudflare-dns" {

					if value == "true" {

						// check if type equals LoadBalancer
						if *service.Spec.Type == "LoadBalancer" {

							// check if LoadBalancer has an ip address
							if len(service.Status.LoadBalancer.Ingress) > 0 {

								fmt.Printf("ip of loadbalancer=%", service.Status.LoadBalancer.Ingress[0].Ip)

							}

						}

					}

					break
				}
			}
		}

	}
}
