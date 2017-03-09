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

	// seed random number
	r = rand.New(rand.NewSource(time.Now().UnixNano()))
)

func init() {
	// Metrics have to be registered to be exposed:
	prometheus.MustRegister(dnsRecordsMutations)
}

func main() {

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

	// start prometheus
	go func() {
		fmt.Println("Serving Prometheus metrics at :9101/metrics...")
		flag.Parse()
		http.Handle("/metrics", promhttp.Handler())
		log.Fatal(http.ListenAndServe(*addr, nil))
	}()

	// watch services for all namespaces
	go func() {
		// loop indefinitely
		for {
			fmt.Println("Watching services for all namespaces...")
			watcher, err := client.CoreV1().WatchServices(context.Background(), k8s.AllNamespaces)
			if err != nil {
				log.Println(err)
			} else {
				// loop indefinitely, unless it errors
				for {
					event, service, err := watcher.Next()
					if err != nil {
						log.Println(err)
						break
					}

					if *event.Type == k8s.EventAdded || *event.Type == k8s.EventModified {
						err := processService(cf, client, service, fmt.Sprintf("watcher:%v", *event.Type))
						if err != nil {
							continue
						}
					}
				}
			}

			// sleep random time between 22 and 37 seconds
			sleepTime := applyJitter(30)
			fmt.Printf("Sleeping for %v seconds...\n", sleepTime)
			time.Sleep(time.Duration(sleepTime) * time.Second)
		}
	}()

	// loop indefinitely
	for {

		// get services for all namespaces
		fmt.Println("Listing services for all namespaces...")
		services, err := client.CoreV1().ListServices(context.Background(), k8s.AllNamespaces)
		if err != nil {
			log.Println(err)
		}
		fmt.Printf("Cluster has %v services\n", len(services.Items))

		// loop all services
		if services != nil && services.Items != nil {
			for _, service := range services.Items {

				err := processService(cf, client, service, "poller")
				if err != nil {
					continue
				}
			}
		}

		// sleep random time between 225 and 375 seconds
		sleepTime := applyJitter(300)
		fmt.Printf("Sleeping for %v seconds...\n", sleepTime)
		time.Sleep(time.Duration(sleepTime) * time.Second)
	}
}

func applyJitter(input int) (output int) {

	deviation := int(0.25 * float64(input))

	return input - deviation + r.Intn(2*deviation)
}

func processService(cf *Cloudflare, client *k8s.Client, service *apiv1.Service, initiator string) error {

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

			serviceIPAddress := *service.Status.LoadBalancer.Ingress[0].Ip

			// update dns record if anything has changed compared to the stored state
			if serviceIPAddress != kubeCloudflareState.IPAddress ||
				kubeCloudflareHostnames != kubeCloudflareState.Hostnames ||
				kubeCloudflareUseOriginRecord != kubeCloudflareState.UseOriginRecord ||
				kubeCloudflareProxy != kubeCloudflareState.Proxy ||
				kubeCloudflareOriginRecordHostname != kubeCloudflareState.OriginRecordHostname {

				// if use origin is enabled, create an A record for the origin
				if kubeCloudflareUseOriginRecord == "true" && kubeCloudflareOriginRecordHostname != "" {

					fmt.Printf("[%v] Service %v.%v - Upserting origin dns record %v (A) to ip address %v...\n", initiator, *service.Metadata.Name, *service.Metadata.Namespace, kubeCloudflareOriginRecordHostname, serviceIPAddress)

					_, err := cf.UpsertDNSRecord("A", kubeCloudflareOriginRecordHostname, serviceIPAddress)
					if err != nil {
						log.Println(err)
						return err
					}
				}

				// loop all hostnames
				hostnames := strings.Split(kubeCloudflareHostnames, ",")
				for _, hostname := range hostnames {

					// if use origin is enabled, create a CNAME record pointing to the origin record
					if kubeCloudflareUseOriginRecord == "true" && kubeCloudflareOriginRecordHostname != "" {

						fmt.Printf("[%v] Service %v.%v - Upserting dns record %v (CNAME) to value %v...\n", initiator, *service.Metadata.Name, *service.Metadata.Namespace, hostname, kubeCloudflareOriginRecordHostname)

						_, err := cf.UpsertDNSRecord("CNAME", hostname, kubeCloudflareOriginRecordHostname)
						if err != nil {
							log.Println(err)
							return err
						}
					} else {

						fmt.Printf("[%v] Service %v.%v - Upserting dns record %v (A) to ip address %v...\n", initiator, *service.Metadata.Name, *service.Metadata.Namespace, hostname, serviceIPAddress)

						_, err := cf.UpsertDNSRecord("A", hostname, serviceIPAddress)
						if err != nil {
							log.Println(err)
							return err
						}
					}

					// if proxy is enabled, update it at Cloudflare
					if kubeCloudflareProxy == "true" {
						fmt.Printf("[%v] Service %v.%v - Enabling proxying for dns record %v (A)...\n", initiator, *service.Metadata.Name, *service.Metadata.Namespace, hostname)
					} else {
						fmt.Printf("[%v] Service %v.%v - Disabling proxying for dns record %v (A)...\n", initiator, *service.Metadata.Name, *service.Metadata.Namespace, hostname)
					}

					_, err := cf.UpdateProxySetting(hostname, kubeCloudflareProxy)
					if err != nil {
						log.Println(err)
						return err
					}
				}

				// if use origin is disabled, remove the A record for the origin
				if kubeCloudflareUseOriginRecord != "true" || kubeCloudflareOriginRecordHostname == "" {

					fmt.Printf("[%v] Service %v.%v - Deleting origin dns record %v (A)...\n", initiator, *service.Metadata.Name, *service.Metadata.Namespace, kubeCloudflareOriginRecordHostname)

					_, err := cf.DeleteDNSRecord(kubeCloudflareOriginRecordHostname)
					if err != nil {
						log.Println(err)
						return err
					}
				}

				// if any state property changed make sure to update all
				kubeCloudflareState.Proxy = kubeCloudflareProxy
				kubeCloudflareState.IPAddress = serviceIPAddress
				kubeCloudflareState.Hostnames = kubeCloudflareHostnames
				kubeCloudflareState.UseOriginRecord = kubeCloudflareUseOriginRecord
				kubeCloudflareState.OriginRecordHostname = kubeCloudflareOriginRecordHostname

				fmt.Printf("[%v] Service %v.%v - Updating service because state has changed...\n", initiator, *service.Metadata.Name, *service.Metadata.Namespace)

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
				fmt.Printf("[%v] Service %v.%v - Service has been updated successfully...\n", initiator, *service.Metadata.Name, *service.Metadata.Namespace)
			}
		}
	}

	return nil
}
