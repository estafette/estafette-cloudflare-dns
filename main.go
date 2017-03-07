package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/ericchiang/k8s"
)

func main() {

	// seed random number
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	// create kubernetes api client
	client, err := k8s.NewInClusterClient()
	if err != nil {
		log.Fatal(err)
	}

	// loop indefinitely
	for {

		// fetch all namespaces from cluster
		fmt.Println("Listing all namespaces...")
		namespaces, err := client.CoreV1().ListNamespaces(context.Background())
		if err != nil {
			log.Fatal(err)
		}

		// loop all namespaces
		for _, namespace := range namespaces.Items {

			// get all services for namespace
			fmt.Printf("Listing all services for namespace %v...\n", *namespace.Metadata.Name)
			services, err := client.CoreV1().ListServices(context.Background(), *namespace.Metadata.Name)
			if err != nil {
				log.Fatal(err)
			}

			// loop all namespaces
			for _, service := range services.Items {

				if &service != nil && &service.Metadata != nil && &service.Metadata.Annotations != nil {

					fmt.Printf("Checking service %v (namespace %v) for travix.io/kube-cloudflare-dns annotation...\n", *service.Metadata.Name, *namespace.Metadata.Name)

					for key, value := range service.Metadata.Annotations {

						// travix.io/kube-cloudflare-dns: "${CLOUDFLARE_CREATE_DNS_RECORD}"
						// travix.io/kube-cloudflare-proxy: "${CLOUDFLARE_ENABLE_PROXY}"
						// travix.io/kube-cloudflare-use-origin-record: "${CLOUDFLARE_USE_ORIGIN_AND_CNAME_DNS_RECORDS}"
						// travix.io/kube-cloudflare-hostnames: "${HOSTNAMES}"

						if key == "travix.io/kube-cloudflare-dns" {

							fmt.Printf("Service %v (namespace %v) has travix.io/kube-cloudflare-dns annotation with value %t...\n", *service.Metadata.Name, *namespace.Metadata.Name, value)

							if value == "true" {

								// check if type equals LoadBalancer
								if *service.Spec.Type == "LoadBalancer" {

									fmt.Printf("Service %v (namespace %v) has type LoadBalancer...\n", *service.Metadata.Name, *namespace.Metadata.Name)

									// check if LoadBalancer has an ip address
									if len(service.Status.LoadBalancer.Ingress) > 0 {

										fmt.Printf("Service %v (namespace %v) has type LoadBalancer ip address %v...\n", *service.Metadata.Name, *namespace.Metadata.Name, *service.Status.LoadBalancer.Ingress[0].Ip)

									}

								}

							}

							// stop inspecting further annotations
							break
						}
					}
				}
			}
		}

		// sleep random time between 20 and 40 seconds
		sleepTime := 20 + r.Intn(20)
		fmt.Printf("Sleeping for %v seconds...\n", sleepTime)
		time.Sleep(time.Duration(sleepTime) * time.Second)
	}
}
