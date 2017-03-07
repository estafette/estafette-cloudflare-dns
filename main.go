package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/ericchiang/k8s"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	addr = flag.String("listen-address", ":9101", "The address to listen on for HTTP requests.")

	dnsRecordsMutations = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "estafette_dns_record_mutations",
			Help: "Number of dns records created or updated.",
		},
		[]string{"device"},
	)
)

func init() {
	// Metrics have to be registered to be exposed:
	prometheus.MustRegister(dnsRecordsMutations)
}

func main() {

	// start prometheus
	go func() {
		fmt.Println("Serving Prometheus metrics at :9101/metrics...")
		flag.Parse()
		http.Handle("/metrics", promhttp.Handler())
		log.Fatal(http.ListenAndServe(*addr, nil))
	}()

	// seed random number
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	// create cloudflare api client
	cf := New(APIAuthentication{Key: os.Getenv("CF_API_KEY"), Email: os.Getenv("CF_API_EMAIL")})

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
		fmt.Printf("Cluster has %v services\n", len(namespaces.Items))

		// loop all namespaces
		for _, namespace := range namespaces.Items {

			// get all services for namespace
			fmt.Printf("Listing all services for namespace %v...\n", *namespace.Metadata.Name)
			services, err := client.CoreV1().ListServices(context.Background(), *namespace.Metadata.Name)
			if err != nil {
				log.Fatal(err)
			}
			fmt.Printf("Namespace %v has %v services\n", *namespace.Metadata.Name, len(services.Items))

			// loop all namespaces
			for _, service := range services.Items {

				if &service != nil && &service.Metadata != nil && &service.Metadata.Annotations != nil {

					fmt.Printf("Checking service %v (namespace %v) for travix.io/kube-cloudflare-dns annotation...\n", *service.Metadata.Name, *namespace.Metadata.Name)

					// get annotations or set default value
					kubeCloudflareDNS, ok := service.Metadata.Annotations["travix.io/kube-cloudflare-dns"]
					if !ok {
						kubeCloudflareDNS = "false"
					}

					kubeCloudflareHostnames, ok := service.Metadata.Annotations["travix.io/kube-cloudflare-hostnames"]
					if !ok {
						kubeCloudflareHostnames = ""
					}

					kubeCloudflareProxy, ok := service.Metadata.Annotations["travix.io/kube-cloudflare-proxy"]
					if !ok {
						kubeCloudflareProxy = "true"
					}

					kubeCloudflareUseOriginRecord, ok := service.Metadata.Annotations["kube-cloudflare-use-origin-record:"]
					if !ok {
						kubeCloudflareUseOriginRecord = "false"
					}

					// check if service has travix.io/kube-cloudflare-dns annotation and it's value is true and
					// check if service has travix.io/kube-cloudflare-hostnames annotation and it's value is not empty and
					// check if type equals LoadBalancer and
					// check if LoadBalancer has an ip address
					if kubeCloudflareDNS == "true" && len(kubeCloudflareHostnames) > 0 && *service.Spec.Type == "LoadBalancer" && len(service.Status.LoadBalancer.Ingress) > 0 {

						// loop all hostnames
						hostnames := strings.Split(kubeCloudflareHostnames, ",")
						for _, hostname := range hostnames {

							fmt.Printf("Updating dns record %v (A) to ip address %v...\n", hostname, *service.Status.LoadBalancer.Ingress[0].Ip)

							_, err := cf.UpsertDNSRecord("A", hostname, *service.Status.LoadBalancer.Ingress[0].Ip)
							if err != nil {
								log.Fatal(err)
							}

							if kubeCloudflareProxy == "true" {
								fmt.Printf("Enabling proxying for dns record %v (A)...\n", hostname)
							}
							if kubeCloudflareUseOriginRecord == "true" {
								fmt.Printf("Using origin dns record for dns record %v (A)...\n", hostname)
							}

							//dnsRecordsMutations.With(prometheus.Labels{"action": "update", "namespace": *namespace.Metadata.Name}).Inc()
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
