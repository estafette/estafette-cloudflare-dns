package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/ericchiang/k8s"
	apiv1 "github.com/ericchiang/k8s/api/v1"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const annotationKubeCloudflareDNS string = "estafette.io/cloudflare-dns"
const annotationKubeCloudflareHostnames string = "estafette.io/cloudflare-hostnames"
const annotationKubeCloudflareProxy string = "estafette.io/cloudflare-proxy"
const annotationKubeCloudflareUseOriginRecord string = "estafette.io/cloudflare-use-origin-record"
const annotationKubeCloudflareOriginRecordHostname string = "estafette.io/cloudflare-origin-record-hostname"

const annotationKubeCloudflareState string = "estafette.io/cloudflare-state"

// KubeCloudflareState represents the state of the service at Cloudflare
type KubeCloudflareState struct {
	Hostnames            string `json:"hostnames"`
	Proxy                string `json:"proxy"`
	UseOriginRecord      string `json:"useOriginRecord"`
	OriginRecordHostname string `json:"originRecordHostname"`
	IPAddress            string `json:"ipAddress"`
}

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

	cfAPIKey := os.Getenv("CF_API_KEY")
	if cfAPIKey == "" {
		log.Fatal("CF_API_KEY is required. Please set CF_API_KEY environment variable to your Cloudflare API key.")
	}
	cfAPIEmail := os.Getenv("CF_API_EMAIL")
	if cfAPIEmail == "" {
		log.Fatal("CF_API_EMAIL is required. Please set CF_API_KEY environment variable to your Cloudflare API email.")
	}

	cf := New(APIAuthentication{Key: cfAPIKey, Email: cfAPIEmail})

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
			log.Println(err)
		}
		fmt.Printf("Cluster has %v services\n", len(namespaces.Items))

		// loop all namespaces
		if namespaces != nil && namespaces.Items != nil {
			for _, namespace := range namespaces.Items {

				// get all services for namespace
				fmt.Printf("Listing all services for namespace %v...\n", *namespace.Metadata.Name)
				services, err := client.CoreV1().ListServices(context.Background(), *namespace.Metadata.Name)
				if err != nil {
					log.Println(err)
				}
				fmt.Printf("Namespace %v has %v services\n", *namespace.Metadata.Name, len(services.Items))

				// loop all services
				if services != nil && services.Items != nil {
					for _, service := range services.Items {

						processService(cf, client, service)
						if err != nil {
							continue
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

func processService(cf *Cloudflare, client *k8s.Client, service *apiv1.Service) error {

	if &service != nil && &service.Metadata != nil && &service.Metadata.Annotations != nil {

		// get annotations or set default value
		kubeCloudflareDNS, ok := service.Metadata.Annotations[annotationKubeCloudflareDNS]
		if !ok {
			kubeCloudflareDNS = "false"
		}
		kubeCloudflareHostnames, ok := service.Metadata.Annotations[annotationKubeCloudflareHostnames]
		if !ok {
			kubeCloudflareHostnames = ""
		}
		kubeCloudflareProxy, ok := service.Metadata.Annotations[annotationKubeCloudflareProxy]
		if !ok {
			kubeCloudflareProxy = "true"
		}
		kubeCloudflareUseOriginRecord, ok := service.Metadata.Annotations[annotationKubeCloudflareUseOriginRecord]
		if !ok {
			kubeCloudflareUseOriginRecord = "false"
		}
		kubeCloudflareOriginRecordHostname, ok := service.Metadata.Annotations[annotationKubeCloudflareOriginRecordHostname]
		if !ok {
			kubeCloudflareOriginRecordHostname = ""
		}

		// get state stored in annotations if present or set to empty struct
		var kubeCloudflareState KubeCloudflareState
		kubeCloudflareStateString, ok := service.Metadata.Annotations[annotationKubeCloudflareState]
		if err := json.Unmarshal([]byte(kubeCloudflareStateString), &kubeCloudflareState); err != nil {
			// couldn't deserialize, setting to default struct
			kubeCloudflareState = KubeCloudflareState{}
		}

		// check if service has travix.io/kube-cloudflare-dns annotation and it's value is true and
		// check if service has travix.io/kube-cloudflare-hostnames annotation and it's value is not empty and
		// check if type equals LoadBalancer and
		// check if LoadBalancer has an ip address
		if kubeCloudflareDNS == "true" && len(kubeCloudflareHostnames) > 0 && *service.Spec.Type == "LoadBalancer" && len(service.Status.LoadBalancer.Ingress) > 0 {

			updateService := false
			serviceIPAddress := *service.Status.LoadBalancer.Ingress[0].Ip

			// kubeCloudflareOriginRecordZone

			if kubeCloudflareUseOriginRecord != kubeCloudflareState.UseOriginRecord && kubeCloudflareUseOriginRecord == "true" && kubeCloudflareOriginRecordHostname != "" {
				fmt.Printf("Updating origin dns record %v (A) to ip address %v...\n", kubeCloudflareOriginRecordHostname, serviceIPAddress)

				_, err := cf.UpsertDNSRecord("A", kubeCloudflareOriginRecordHostname, serviceIPAddress)
				if err != nil {
					log.Println(err)
					return err
				}

				// set state annotation
				kubeCloudflareState.UseOriginRecord = kubeCloudflareUseOriginRecord
				updateService = true
			} else {
				fmt.Printf("Skip updating origin dns record %v because state hasn't changed...\n", kubeCloudflareOriginRecordHostname)
			}

			// loop all hostnames
			hostnames := strings.Split(kubeCloudflareHostnames, ",")
			for _, hostname := range hostnames {

				// update dns record if it's new or has changed or there are new hosts
				if serviceIPAddress != kubeCloudflareState.IPAddress || kubeCloudflareHostnames != kubeCloudflareState.Hostnames || kubeCloudflareUseOriginRecord != kubeCloudflareState.UseOriginRecord {

					if kubeCloudflareUseOriginRecord != kubeCloudflareState.UseOriginRecord && kubeCloudflareUseOriginRecord == "true" && kubeCloudflareOriginRecordHostname != "" {

						fmt.Printf("Updating dns record %v (CNAME) to value %v...\n", hostname, kubeCloudflareOriginRecordHostname)

						_, err := cf.UpsertDNSRecord("CNAME", hostname, kubeCloudflareOriginRecordHostname)
						if err != nil {
							log.Println(err)
							return err
						}

					} else {

						fmt.Printf("Updating dns record %v (A) to ip address %v...\n", hostname, serviceIPAddress)

						_, err := cf.UpsertDNSRecord("A", hostname, serviceIPAddress)
						if err != nil {
							log.Println(err)
							continue
						}

					}

					// set state annotation
					kubeCloudflareState.IPAddress = serviceIPAddress
					kubeCloudflareState.Hostnames = kubeCloudflareHostnames
					updateService = true
				} else {
					fmt.Printf("Skip updating dns record %v (A) because state hasn't changed...\n", hostname)
				}

				if kubeCloudflareProxy != kubeCloudflareState.Proxy {
					if kubeCloudflareProxy == "true" {
						fmt.Printf("Enabling proxying for dns record %v (A)...\n", hostname)
					} else {
						fmt.Printf("Disabling proxying for dns record %v (A)...\n", hostname)
					}

					_, err := cf.UpdateProxySetting(hostname, kubeCloudflareProxy)
					if err != nil {
						log.Println(err)
						return err
					}

					// set state annotation
					kubeCloudflareState.Proxy = kubeCloudflareProxy
					updateService = true
				} else {
					fmt.Printf("Skip updating dns record %v proxied setting because state hasn't changed...\n", hostname)
				}

				//dnsRecordsMutations.With(prometheus.Labels{"action": "update", "namespace": *namespace.Metadata.Name}).Inc()
			}

			if updateService {

				fmt.Printf("Updating service %v (namespace %v) because state has changed...\n", *service.Metadata.Name, *service.Metadata.Namespace)

				// serialize state and store it in the annotation
				kubeCloudflareStateByteArray, err := json.Marshal(kubeCloudflareState)
				if err != nil {
					log.Println(err)
					return err
				}
				service.Metadata.Annotations[annotationKubeCloudflareState] = string(kubeCloudflareStateByteArray)

				// update service, because the state annotations have changed
				service, err = client.CoreV1().UpdateService(context.Background(), service)
				if err != nil {
					log.Println(err)
					return err
				}
			}
		}
	}

	return nil
}
