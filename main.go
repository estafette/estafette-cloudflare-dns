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

const annotationCloudflareDNS string = "estafette.io/cloudflare-dns"
const annotationCloudflareHostnames string = "estafette.io/cloudflare-hostnames"
const annotationCloudflareProxy string = "estafette.io/cloudflare-proxy"
const annotationCloudflareUseOriginRecord string = "estafette.io/cloudflare-use-origin-record"
const annotationCloudflareOriginRecordHostname string = "estafette.io/cloudflare-origin-record-hostname"

const annotationCloudflareState string = "estafette.io/cloudflare-state"

// CloudflareState represents the state of the service at Cloudflare
type CloudflareState struct {
	Hostnames            string `json:"hostnames"`
	Proxy                string `json:"proxy"`
	UseOriginRecord      string `json:"useOriginRecord"`
	OriginRecordHostname string `json:"originRecordHostname"`
	IPAddress            string `json:"ipAddress"`
}

var (
	addr = flag.String("listen-address", ":9101", "The address to listen on for HTTP requests.")

	// seed random number
	r = rand.New(rand.NewSource(time.Now().UnixNano()))

	// define prometheus counter
	dnsRecordsTotals = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "estafette_cloudflare_dns_record_totals",
			Help: "Number of updated Cloudflare dns records.",
		},
		[]string{"namespace", "status", "initiator"},
	)
)

func init() {
	// Metrics have to be registered to be exposed:
	prometheus.MustRegister(dnsRecordsTotals)
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
						status, err := processService(cf, client, service, fmt.Sprintf("watcher:%v", *event.Type))
						dnsRecordsTotals.With(prometheus.Labels{"namespace": *service.Metadata.Namespace, "status": status, "initiator": "watcher"}).Inc()
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

				status, err := processService(cf, client, service, "poller")
				dnsRecordsTotals.With(prometheus.Labels{"namespace": *service.Metadata.Namespace, "status": status, "initiator": "poller"}).Inc()
				if err != nil {
					continue
				}
			}
		}

		// sleep random time around 900 seconds
		sleepTime := applyJitter(900)
		fmt.Printf("Sleeping for %v seconds...\n", sleepTime)
		time.Sleep(time.Duration(sleepTime) * time.Second)
	}
}

func applyJitter(input int) (output int) {

	deviation := int(0.25 * float64(input))

	return input - deviation + r.Intn(2*deviation)
}

func processService(cf *Cloudflare, client *k8s.Client, service *apiv1.Service, initiator string) (status string, err error) {

	status = "failed"

	if &service != nil && &service.Metadata != nil && &service.Metadata.Annotations != nil {

		// get annotations or set default value
		cloudflareDNS, ok := service.Metadata.Annotations[annotationCloudflareDNS]
		if !ok {
			cloudflareDNS = "false"
		}
		cloudflareHostnames, ok := service.Metadata.Annotations[annotationCloudflareHostnames]
		if !ok {
			cloudflareHostnames = ""
		}
		cloudflareProxy, ok := service.Metadata.Annotations[annotationCloudflareProxy]
		if !ok {
			cloudflareProxy = "true"
		}
		cloudflareUseOriginRecord, ok := service.Metadata.Annotations[annotationCloudflareUseOriginRecord]
		if !ok {
			cloudflareUseOriginRecord = "false"
		}
		cloudflareOriginRecordHostname, ok := service.Metadata.Annotations[annotationCloudflareOriginRecordHostname]
		if !ok {
			cloudflareOriginRecordHostname = ""
		}

		// get state stored in annotations if present or set to empty struct
		var cloudflareState CloudflareState
		cloudflareStateString, ok := service.Metadata.Annotations[annotationCloudflareState]
		if err := json.Unmarshal([]byte(cloudflareStateString), &cloudflareState); err != nil {
			// couldn't deserialize, setting to default struct
			cloudflareState = CloudflareState{}
		}

		// check if service has estafette.io/cloudflare-dns annotation and it's value is true and
		// check if service has estafette.io/cloudflare-hostnames annotation and it's value is not empty and
		// check if type equals LoadBalancer and
		// check if LoadBalancer has an ip address
		if cloudflareDNS == "true" && len(cloudflareHostnames) > 0 && *service.Spec.Type == "LoadBalancer" && len(service.Status.LoadBalancer.Ingress) > 0 {

			serviceIPAddress := *service.Status.LoadBalancer.Ingress[0].Ip

			// update dns record if anything has changed compared to the stored state
			if serviceIPAddress != cloudflareState.IPAddress ||
				cloudflareHostnames != cloudflareState.Hostnames ||
				cloudflareUseOriginRecord != cloudflareState.UseOriginRecord ||
				cloudflareProxy != cloudflareState.Proxy ||
				cloudflareOriginRecordHostname != cloudflareState.OriginRecordHostname {

				// if use origin is enabled, create an A record for the origin
				if cloudflareUseOriginRecord == "true" && cloudflareOriginRecordHostname != "" {

					fmt.Printf("[%v] Service %v.%v - Upserting origin dns record %v (A) to ip address %v...\n", initiator, *service.Metadata.Name, *service.Metadata.Namespace, cloudflareOriginRecordHostname, serviceIPAddress)

					_, err := cf.UpsertDNSRecord("A", cloudflareOriginRecordHostname, serviceIPAddress)
					if err != nil {
						log.Println(err)
						return status, err
					}
				}

				// loop all hostnames
				hostnames := strings.Split(cloudflareHostnames, ",")
				for _, hostname := range hostnames {

					// if use origin is enabled, create a CNAME record pointing to the origin record
					if cloudflareUseOriginRecord == "true" && cloudflareOriginRecordHostname != "" {

						fmt.Printf("[%v] Service %v.%v - Upserting dns record %v (CNAME) to value %v...\n", initiator, *service.Metadata.Name, *service.Metadata.Namespace, hostname, cloudflareOriginRecordHostname)

						_, err := cf.UpsertDNSRecord("CNAME", hostname, cloudflareOriginRecordHostname)
						if err != nil {
							log.Println(err)
							return status, err
						}
					} else {

						fmt.Printf("[%v] Service %v.%v - Upserting dns record %v (A) to ip address %v...\n", initiator, *service.Metadata.Name, *service.Metadata.Namespace, hostname, serviceIPAddress)

						_, err := cf.UpsertDNSRecord("A", hostname, serviceIPAddress)
						if err != nil {
							log.Println(err)
							return status, err
						}
					}

					// if proxy is enabled, update it at Cloudflare
					if cloudflareProxy == "true" {
						fmt.Printf("[%v] Service %v.%v - Enabling proxying for dns record %v (A)...\n", initiator, *service.Metadata.Name, *service.Metadata.Namespace, hostname)
					} else {
						fmt.Printf("[%v] Service %v.%v - Disabling proxying for dns record %v (A)...\n", initiator, *service.Metadata.Name, *service.Metadata.Namespace, hostname)
					}

					_, err := cf.UpdateProxySetting(hostname, cloudflareProxy)
					if err != nil {
						log.Println(err)
						return status, err
					}
				}

				// if use origin is disabled, remove the A record for the origin, if state still has a value for OriginRecordHostname
				if cloudflareState.OriginRecordHostname != "" && (cloudflareUseOriginRecord != "true" || cloudflareOriginRecordHostname == "") {

					fmt.Printf("[%v] Service %v.%v - Deleting origin dns record %v (A)...\n", initiator, *service.Metadata.Name, *service.Metadata.Namespace, cloudflareOriginRecordHostname)

					_, err := cf.DeleteDNSRecord(cloudflareState.OriginRecordHostname)
					if err != nil {
						log.Println(err)
						return status, err
					}
				}

				// if any state property changed make sure to update all
				cloudflareState.Proxy = cloudflareProxy
				cloudflareState.IPAddress = serviceIPAddress
				cloudflareState.Hostnames = cloudflareHostnames
				cloudflareState.UseOriginRecord = cloudflareUseOriginRecord
				cloudflareState.OriginRecordHostname = cloudflareOriginRecordHostname

				fmt.Printf("[%v] Service %v.%v - Updating service because state has changed...\n", initiator, *service.Metadata.Name, *service.Metadata.Namespace)

				// serialize state and store it in the annotation
				cloudflareStateByteArray, err := json.Marshal(cloudflareState)
				if err != nil {
					log.Println(err)
					return status, err
				}
				service.Metadata.Annotations[annotationCloudflareState] = string(cloudflareStateByteArray)

				// update service, because the state annotations have changed
				service, err = client.CoreV1().UpdateService(context.Background(), service)
				if err != nil {
					log.Println(err)
					return status, err
				}

				status = "succeeded"

				fmt.Printf("[%v] Service %v.%v - Service has been updated successfully...\n", initiator, *service.Metadata.Name, *service.Metadata.Namespace)
			}
		}
	}

	status = "skipped"

	return status, nil
}
