package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	stdlog "log"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/ericchiang/k8s"
	apiv1 "github.com/ericchiang/k8s/api/v1"
	extensionsv1beta1 "github.com/ericchiang/k8s/apis/extensions/v1beta1"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const annotationCloudflareDNS string = "estafette.io/cloudflare-dns"
const annotationCloudflareHostnames string = "estafette.io/cloudflare-hostnames"
const annotationCloudflareInternalHostnames string = "estafette.io/cloudflare-internal-hostnames"
const annotationCloudflareProxy string = "estafette.io/cloudflare-proxy"
const annotationCloudflareUseOriginRecord string = "estafette.io/cloudflare-use-origin-record"
const annotationCloudflareOriginRecordHostname string = "estafette.io/cloudflare-origin-record-hostname"

const annotationCloudflareState string = "estafette.io/cloudflare-state"

// CloudflareState represents the state of the service at Cloudflare
type CloudflareState struct {
	Enabled              string `json:"enabled"`
	Hostnames            string `json:"hostnames"`
	InternalHostnames    string `json:"internalHostnames,omitempty"`
	Proxy                string `json:"proxy"`
	UseOriginRecord      string `json:"useOriginRecord"`
	OriginRecordHostname string `json:"originRecordHostname"`
	IPAddress            string `json:"ipAddress"`
	InternalIPAddress    string `json:"internalIpAddress,omitempty"`
}

var (
	version   string
	branch    string
	revision  string
	buildDate string
	goVersion = runtime.Version()
)

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
		[]string{"namespace", "status", "initiator", "type"},
	)
)

func init() {
	// Metrics have to be registered to be exposed:
	prometheus.MustRegister(dnsRecordsTotals)
}

func main() {

	// parse command line parameters
	flag.Parse()

	// log as severity for stackdriver logging to recognize the level
	zerolog.LevelFieldName = "severity"

	// set some default fields added to all logs
	log.Logger = zerolog.New(os.Stdout).With().
		Timestamp().
		Str("app", "estafette-cloudflare-dns").
		Str("version", version).
		Logger()

	// use zerolog for any logs sent via standard log library
	stdlog.SetFlags(0)
	stdlog.SetOutput(log.Logger)

	// log startup message
	log.Info().
		Str("branch", branch).
		Str("revision", revision).
		Str("buildDate", buildDate).
		Str("goVersion", goVersion).
		Msg("Starting estafette-cloudflare-dns...")

	// create cloudflare api client
	cfAPIKey := os.Getenv("CF_API_KEY")
	if cfAPIKey == "" {
		log.Fatal().Msg("CF_API_KEY is required. Please set CF_API_KEY environment variable to your Cloudflare API key.")
	}
	cfAPIEmail := os.Getenv("CF_API_EMAIL")
	if cfAPIEmail == "" {
		log.Fatal().Msg("CF_API_EMAIL is required. Please set CF_API_KEY environment variable to your Cloudflare API email.")
	}

	cf := New(APIAuthentication{Key: cfAPIKey, Email: cfAPIEmail})

	// create kubernetes api client
	client, err := k8s.NewInClusterClient()
	if err != nil {
		log.Fatal().Err(err).Msg("Creating Kubernetes api client failed")
	}

	// start prometheus
	go func() {
		log.Debug().
			Str("port", *addr).
			Msg("Serving Prometheus metrics...")

		http.Handle("/metrics", promhttp.Handler())

		if err := http.ListenAndServe(*addr, nil); err != nil {
			log.Fatal().Err(err).Msg("Starting Prometheus listener failed")
		}
	}()

	// define channel and wait group to gracefully shutdown the application
	gracefulShutdown := make(chan os.Signal)
	signal.Notify(gracefulShutdown, syscall.SIGTERM, syscall.SIGINT)
	waitGroup := &sync.WaitGroup{}

	// watch services for all namespaces
	go func(waitGroup *sync.WaitGroup) {
		// loop indefinitely
		for {
			log.Info().Msg("Watching services for all namespaces...")
			watcher, err := client.CoreV1().WatchServices(context.Background(), k8s.AllNamespaces)
			if err != nil {
				log.Error().Err(err).Msg("WatchServices call failed")
			} else {
				// loop indefinitely, unless it errors
				for {
					event, service, err := watcher.Next()
					if err != nil {
						log.Error().Err(err).Msg("Getting next event from service watcher failed")
						break
					}

					if *event.Type == k8s.EventAdded || *event.Type == k8s.EventModified {
						waitGroup.Add(1)
						status, err := processService(cf, client, service, fmt.Sprintf("watcher:%v", *event.Type))
						dnsRecordsTotals.With(prometheus.Labels{"namespace": *service.Metadata.Namespace, "status": status, "initiator": "watcher", "type": "service"}).Inc()
						waitGroup.Done()

						if err != nil {
							log.Error().Err(err).Msgf("Processing service %v.%v failed", *service.Metadata.Name, *service.Metadata.Namespace)
							continue
						}
					}
				}
			}

			// sleep random time between 22 and 37 seconds
			sleepTime := applyJitter(30)
			log.Info().Msgf("Sleeping for %v seconds...", sleepTime)
			time.Sleep(time.Duration(sleepTime) * time.Second)
		}
	}(waitGroup)

	// watch ingresses for all namespaces
	go func(waitGroup *sync.WaitGroup) {
		// loop indefinitely
		for {
			log.Info().Msg("Watching ingresses for all namespaces...")
			watcher, err := client.ExtensionsV1Beta1().WatchIngresses(context.Background(), k8s.AllNamespaces)
			if err != nil {
				log.Error().Err(err).Msg("WatchIngresses call failed")
			} else {
				// loop indefinitely, unless it errors
				for {
					event, ingress, err := watcher.Next()
					if err != nil {
						log.Error().Err(err).Msg("Getting next event from ingress watcher failed")
						break
					}

					if *event.Type == k8s.EventAdded || *event.Type == k8s.EventModified {
						waitGroup.Add(1)
						status, err := processIngress(cf, client, ingress, fmt.Sprintf("watcher:%v", *event.Type))
						dnsRecordsTotals.With(prometheus.Labels{"namespace": *ingress.Metadata.Namespace, "status": status, "initiator": "watcher", "type": "ingress"}).Inc()
						waitGroup.Done()

						if err != nil {
							log.Error().Err(err).Msgf("Processing ingress %v.%v failed", *ingress.Metadata.Name, *ingress.Metadata.Namespace)
							continue
						}
					}
				}
			}

			// sleep random time between 22 and 37 seconds
			sleepTime := applyJitter(30)
			log.Info().Msgf("Sleeping for %v seconds...", sleepTime)
			time.Sleep(time.Duration(sleepTime) * time.Second)
		}
	}(waitGroup)

	go func(waitGroup *sync.WaitGroup) {
		// loop indefinitely
		for {

			// get services for all namespaces
			log.Info().Msg("Listing services for all namespaces...")
			services, err := client.CoreV1().ListServices(context.Background(), k8s.AllNamespaces)
			if err != nil {
				log.Error().Err(err).Msg("ListServices call failed")
			}
			log.Info().Msgf("Cluster has %v services", len(services.Items))

			// loop all services
			if services != nil && services.Items != nil {
				for _, service := range services.Items {

					waitGroup.Add(1)
					status, err := processService(cf, client, service, "poller")
					dnsRecordsTotals.With(prometheus.Labels{"namespace": *service.Metadata.Namespace, "status": status, "initiator": "poller", "type": "service"}).Inc()
					waitGroup.Done()

					if err != nil {
						log.Error().Err(err).Msgf("Processing service %v.%v failed", *service.Metadata.Name, *service.Metadata.Namespace)
						continue
					}
				}
			}

			// get ingresses for all namespaces
			log.Info().Msg("Listing ingresses for all namespaces...")
			ingresses, err := client.ExtensionsV1Beta1().ListIngresses(context.Background(), k8s.AllNamespaces)
			if err != nil {
				log.Error().Err(err).Msg("ListIngresses call failed")
			}
			log.Info().Msgf("Cluster has %v ingresses", len(ingresses.Items))

			// loop all ingresses
			if ingresses != nil && ingresses.Items != nil {
				for _, ingress := range ingresses.Items {

					waitGroup.Add(1)
					status, err := processIngress(cf, client, ingress, "poller")
					dnsRecordsTotals.With(prometheus.Labels{"namespace": *ingress.Metadata.Namespace, "status": status, "initiator": "poller", "type": "ingress"}).Inc()
					waitGroup.Done()

					if err != nil {
						log.Error().Err(err).Msgf("Processing ingress %v.%v failed", *ingress.Metadata.Name, *ingress.Metadata.Namespace)
						continue
					}
				}
			}

			// sleep random time around 900 seconds
			sleepTime := applyJitter(900)
			log.Info().Msgf("Sleeping for %v seconds...", sleepTime)
			time.Sleep(time.Duration(sleepTime) * time.Second)
		}
	}(waitGroup)

	signalReceived := <-gracefulShutdown
	log.Info().
		Msgf("Received signal %v. Waiting on running tasks to finish...", signalReceived)

	waitGroup.Wait()

	log.Info().Msg("Shutting down...")
}

func applyJitter(input int) (output int) {

	deviation := int(0.25 * float64(input))

	return input - deviation + r.Intn(2*deviation)
}

func getDesiredServiceState(service *apiv1.Service) (state CloudflareState) {

	var ok bool

	state.Enabled, ok = service.Metadata.Annotations[annotationCloudflareDNS]
	if !ok {
		state.Enabled = "false"
	}
	state.Hostnames, ok = service.Metadata.Annotations[annotationCloudflareHostnames]
	if !ok {
		state.Hostnames = ""
	}
	state.InternalHostnames, ok = service.Metadata.Annotations[annotationCloudflareInternalHostnames]
	if !ok {
		state.InternalHostnames = ""
	}
	state.Proxy, ok = service.Metadata.Annotations[annotationCloudflareProxy]
	if !ok {
		state.Proxy = "true"
	}
	state.UseOriginRecord, ok = service.Metadata.Annotations[annotationCloudflareUseOriginRecord]
	if !ok {
		state.UseOriginRecord = "false"
	}
	state.OriginRecordHostname, ok = service.Metadata.Annotations[annotationCloudflareOriginRecordHostname]
	if !ok {
		state.OriginRecordHostname = ""
	}

	if *service.Spec.Type == "LoadBalancer" && len(service.Status.LoadBalancer.Ingress) > 0 {
		state.IPAddress = *service.Status.LoadBalancer.Ingress[0].Ip
	}
	if *service.Spec.ClusterIP != "" {
		state.InternalIPAddress = *service.Spec.ClusterIP
	}

	return
}

func getCurrentServiceState(service *apiv1.Service) (state CloudflareState) {

	// get state stored in annotations if present or set to empty struct
	cloudflareStateString, ok := service.Metadata.Annotations[annotationCloudflareState]
	if !ok {
		// couldn't find saved state, setting to default struct
		state = CloudflareState{}
		return
	}

	if err := json.Unmarshal([]byte(cloudflareStateString), &state); err != nil {
		// couldn't deserialize, setting to default struct
		state = CloudflareState{}
		return
	}

	// return deserialized state
	return
}

func makeServiceChanges(cf *Cloudflare, client *k8s.Client, service *apiv1.Service, initiator string, desiredState, currentState CloudflareState) (status string, err error) {

	status = "failed"
	hasChanges := false

	// check if service has estafette.io/cloudflare-dns annotation and it's value is true and
	// check if service has estafette.io/cloudflare-hostnames annotation and it's value is not empty and
	// check if type equals LoadBalancer and
	// check if LoadBalancer has an ip address
	if desiredState.Enabled == "true" && len(desiredState.Hostnames) > 0 && desiredState.IPAddress != "" {

		// update dns record if anything has changed compared to the stored state
		if desiredState.IPAddress != currentState.IPAddress ||
			desiredState.Hostnames != currentState.Hostnames ||
			desiredState.Proxy != currentState.Proxy ||
			desiredState.UseOriginRecord != currentState.UseOriginRecord ||
			desiredState.OriginRecordHostname != currentState.OriginRecordHostname {

			hasChanges = true

			// if use origin is enabled, create an A record for the origin
			if desiredState.UseOriginRecord == "true" && desiredState.OriginRecordHostname != "" {

				log.Info().Msgf("[%v] Service %v.%v - Upserting origin dns record %v (A) to ip address %v...", initiator, *service.Metadata.Name, *service.Metadata.Namespace, desiredState.OriginRecordHostname, desiredState.IPAddress)

				_, err := cf.UpsertDNSRecord("A", desiredState.OriginRecordHostname, desiredState.IPAddress, false)
				if err != nil {
					log.Error().Err(err).Msgf("[%v] Service %v.%v - Upserting origin dns record %v (A) to ip address %v failed", initiator, *service.Metadata.Name, *service.Metadata.Namespace, desiredState.OriginRecordHostname, desiredState.IPAddress)
					return status, err
				}
			}

			// loop all hostnames
			hostnames := strings.Split(desiredState.Hostnames, ",")
			for _, hostname := range hostnames {

				// validate hostname, skip if invalid
				if !validateHostname(hostname) {
					log.Error().Err(err).Msgf("[%v] Service %v.%v - Invalid dns record %v, skipping", initiator, *service.Metadata.Name, *service.Metadata.Namespace, hostname)
					continue
				}

				// if use origin is enabled, create a CNAME record pointing to the origin record
				if desiredState.UseOriginRecord == "true" && desiredState.OriginRecordHostname != "" {

					log.Info().Msgf("[%v] Service %v.%v - Upserting dns record %v (CNAME) to value %v...", initiator, *service.Metadata.Name, *service.Metadata.Namespace, hostname, desiredState.OriginRecordHostname)

					_, err := cf.UpsertDNSRecord("CNAME", hostname, desiredState.OriginRecordHostname, desiredState.Proxy == "true")
					if err != nil {
						log.Error().Err(err).Msgf("[%v] Service %v.%v - Upserting dns record %v (CNAME) to value %v failed", initiator, *service.Metadata.Name, *service.Metadata.Namespace, hostname, desiredState.OriginRecordHostname)
						return status, err
					}
				} else {

					log.Info().Msgf("[%v] Service %v.%v - Upserting dns record %v (A) to ip address %v...", initiator, *service.Metadata.Name, *service.Metadata.Namespace, hostname, desiredState.IPAddress)

					_, err := cf.UpsertDNSRecord("A", hostname, desiredState.IPAddress, desiredState.Proxy == "true")
					if err != nil {
						log.Error().Err(err).Msgf("[%v] Service %v.%v - Upserting dns record %v (A) to ip address %v failed", initiator, *service.Metadata.Name, *service.Metadata.Namespace, hostname, desiredState.IPAddress)
						return status, err
					}
				}

				// if proxy is enabled, update it at Cloudflare
				if desiredState.Proxy == "true" {
					log.Info().Msgf("[%v] Service %v.%v - Enabling proxying for dns record %v (A)...", initiator, *service.Metadata.Name, *service.Metadata.Namespace, hostname)
				} else {
					log.Info().Msgf("[%v] Service %v.%v - Disabling proxying for dns record %v (A)...", initiator, *service.Metadata.Name, *service.Metadata.Namespace, hostname)
				}

				_, err := cf.UpdateProxySetting(hostname, desiredState.Proxy == "true")
				if err != nil {
					if desiredState.Proxy == "true" {
						log.Error().Err(err).Msgf("[%v] Service %v.%v - Enabling proxying for dns record %v (A) failed", initiator, *service.Metadata.Name, *service.Metadata.Namespace, hostname)
					} else {
						log.Error().Err(err).Msgf("[%v] Service %v.%v - Disabling proxying for dns record %v (A) failed", initiator, *service.Metadata.Name, *service.Metadata.Namespace, hostname)
					}

					return status, err
				}
			}

			// if use origin is disabled, remove the A record for the origin, if state still has a value for OriginRecordHostname
			if desiredState.OriginRecordHostname != "" && (desiredState.UseOriginRecord != "true" || desiredState.OriginRecordHostname == "") {

				log.Info().Msgf("[%v] Service %v.%v - Deleting origin dns record %v (A)...", initiator, *service.Metadata.Name, *service.Metadata.Namespace, desiredState.OriginRecordHostname)

				_, err := cf.DeleteDNSRecord(desiredState.OriginRecordHostname)
				if err != nil {
					log.Error().Err(err).Msgf("[%v] Service %v.%v - Deleting origin dns record %v (A) failed", initiator, *service.Metadata.Name, *service.Metadata.Namespace, desiredState.OriginRecordHostname)
					return status, err
				}
			}
		}
	}

	// check if service has estafette.io/cloudflare-dns annotation and it's value is true and
	// check if service has estafette.io/cloudflare-internal-hostnames annotation and it's value is not empty and
	// check if service has an internal ip address
	if desiredState.Enabled == "true" && len(desiredState.InternalHostnames) > 0 && desiredState.InternalIPAddress != "" {

		// update internal dns record if anything has changed compared to the stored state
		if desiredState.InternalIPAddress != currentState.InternalIPAddress ||
			desiredState.InternalHostnames != currentState.InternalHostnames {

			hasChanges = true

			// loop all internal hostnames
			internalHostnames := strings.Split(desiredState.InternalHostnames, ",")
			for _, internalHostname := range internalHostnames {

				log.Info().Msgf("[%v] Service %v.%v - Upserting dns record %v (A) to internal ip address %v...", initiator, *service.Metadata.Name, *service.Metadata.Namespace, internalHostname, desiredState.InternalIPAddress)

				_, err := cf.UpsertDNSRecord("A", internalHostname, desiredState.InternalIPAddress, false)
				if err != nil {
					log.Error().Err(err).Msgf("[%v] Service %v.%v - Upserting dns record %v (A) to internal ip address %v failed", initiator, *service.Metadata.Name, *service.Metadata.Namespace, internalHostname, desiredState.InternalIPAddress)
					return status, err
				}
			}
		}
	}

	if hasChanges {

		// if any state property changed make sure to update all
		currentState = desiredState

		log.Info().Msgf("[%v] Service %v.%v - Updating service because state has changed...", initiator, *service.Metadata.Name, *service.Metadata.Namespace)

		// serialize state and store it in the annotation
		cloudflareStateByteArray, err := json.Marshal(currentState)
		if err != nil {
			log.Error().Err(err).Msgf("[%v] Service %v.%v - Marshalling state failed", initiator, *service.Metadata.Name, *service.Metadata.Namespace)
			return status, err
		}
		service.Metadata.Annotations[annotationCloudflareState] = string(cloudflareStateByteArray)

		// update service, because the state annotations have changed
		service, err = client.CoreV1().UpdateService(context.Background(), service)
		if err != nil {
			log.Error().Err(err).Msgf("[%v] Service %v.%v - Updating service state has failed", initiator, *service.Metadata.Name, *service.Metadata.Namespace)
			return status, err
		}

		status = "succeeded"

		log.Info().Msgf("[%v] Service %v.%v - Service has been updated successfully...", initiator, *service.Metadata.Name, *service.Metadata.Namespace)

		return status, nil
	}

	status = "skipped"

	return status, nil
}

func processService(cf *Cloudflare, client *k8s.Client, service *apiv1.Service, initiator string) (status string, err error) {

	status = "failed"

	if &service != nil && &service.Metadata != nil && &service.Metadata.Annotations != nil {

		desiredState := getDesiredServiceState(service)
		currentState := getCurrentServiceState(service)

		status, err = makeServiceChanges(cf, client, service, initiator, desiredState, currentState)

		return
	}

	status = "skipped"

	return status, nil
}

func getDesiredIngressState(ingress *extensionsv1beta1.Ingress) (state CloudflareState) {

	var ok bool

	state.Enabled, ok = ingress.Metadata.Annotations[annotationCloudflareDNS]
	if !ok {
		state.Enabled = "false"
	}
	state.Hostnames, ok = ingress.Metadata.Annotations[annotationCloudflareHostnames]
	if !ok {
		state.Hostnames = ""
	}
	state.Proxy, ok = ingress.Metadata.Annotations[annotationCloudflareProxy]
	if !ok {
		state.Proxy = "true"
	}
	state.UseOriginRecord, ok = ingress.Metadata.Annotations[annotationCloudflareUseOriginRecord]
	if !ok {
		state.UseOriginRecord = "false"
	}
	state.OriginRecordHostname, ok = ingress.Metadata.Annotations[annotationCloudflareOriginRecordHostname]
	if !ok {
		state.OriginRecordHostname = ""
	}

	if len(ingress.Status.LoadBalancer.Ingress) > 0 {
		state.IPAddress = *ingress.Status.LoadBalancer.Ingress[0].Ip
	}

	return
}

func getCurrentIngressState(ingress *extensionsv1beta1.Ingress) (state CloudflareState) {

	// get state stored in annotations if present or set to empty struct
	cloudflareStateString, ok := ingress.Metadata.Annotations[annotationCloudflareState]
	if !ok {
		// couldn't find saved state, setting to default struct
		state = CloudflareState{}
		return
	}

	if err := json.Unmarshal([]byte(cloudflareStateString), &state); err != nil {
		// couldn't deserialize, setting to default struct
		state = CloudflareState{}
		return
	}

	// return deserialized state
	return
}

func makeIngressChanges(cf *Cloudflare, client *k8s.Client, ingress *extensionsv1beta1.Ingress, initiator string, desiredState, currentState CloudflareState) (status string, err error) {

	status = "failed"

	// check if ingress has estafette.io/cloudflare-dns annotation and it's value is true and
	// check if ingress has estafette.io/cloudflare-hostnames annotation and it's value is not empty and
	// check if type equals LoadBalancer and
	// check if LoadBalancer has an ip address
	if desiredState.Enabled == "true" && len(desiredState.Hostnames) > 0 && desiredState.IPAddress != "" {

		// update dns record if anything has changed compared to the stored state
		if desiredState.IPAddress != currentState.IPAddress ||
			desiredState.Hostnames != currentState.Hostnames ||
			desiredState.Proxy != currentState.Proxy ||
			desiredState.UseOriginRecord != currentState.UseOriginRecord ||
			desiredState.OriginRecordHostname != currentState.OriginRecordHostname {

			// if use origin is enabled, create an A record for the origin
			if desiredState.UseOriginRecord == "true" && desiredState.OriginRecordHostname != "" {

				log.Info().Msgf("[%v] Ingress %v.%v - Upserting origin dns record %v (A) to ip address %v...", initiator, *ingress.Metadata.Name, *ingress.Metadata.Namespace, desiredState.OriginRecordHostname, desiredState.IPAddress)

				_, err := cf.UpsertDNSRecord("A", desiredState.OriginRecordHostname, desiredState.IPAddress, false)
				if err != nil {
					log.Error().Err(err).Msgf("[%v] Ingress %v.%v - Upserting origin dns record %v (A) to ip address %v failed", initiator, *ingress.Metadata.Name, *ingress.Metadata.Namespace, desiredState.OriginRecordHostname, desiredState.IPAddress)
					return status, err
				}
			}

			// loop all hostnames
			hostnames := strings.Split(desiredState.Hostnames, ",")
			for _, hostname := range hostnames {

				// if use origin is enabled, create a CNAME record pointing to the origin record
				if desiredState.UseOriginRecord == "true" && desiredState.OriginRecordHostname != "" {

					log.Info().Msgf("[%v] Ingress %v.%v - Upserting dns record %v (CNAME) to value %v...", initiator, *ingress.Metadata.Name, *ingress.Metadata.Namespace, hostname, desiredState.OriginRecordHostname)

					_, err := cf.UpsertDNSRecord("CNAME", hostname, desiredState.OriginRecordHostname, desiredState.Proxy == "true")
					if err != nil {
						log.Error().Err(err).Msgf("[%v] Ingress %v.%v - Upserting dns record %v (CNAME) to value %v failed", initiator, *ingress.Metadata.Name, *ingress.Metadata.Namespace, hostname, desiredState.OriginRecordHostname)
						return status, err
					}
				} else {

					log.Info().Msgf("[%v] Ingress %v.%v - Upserting dns record %v (A) to ip address %v...", initiator, *ingress.Metadata.Name, *ingress.Metadata.Namespace, hostname, desiredState.IPAddress)

					_, err := cf.UpsertDNSRecord("A", hostname, desiredState.IPAddress, desiredState.Proxy == "true")
					if err != nil {
						log.Error().Err(err).Msgf("[%v] Ingress %v.%v - Upserting dns record %v (A) to ip address %v failed", initiator, *ingress.Metadata.Name, *ingress.Metadata.Namespace, hostname, desiredState.IPAddress)
						return status, err
					}
				}

				// if proxy is enabled, update it at Cloudflare
				if desiredState.Proxy == "true" {
					log.Info().Msgf("[%v] Ingress %v.%v - Enabling proxying for dns record %v (A)...", initiator, *ingress.Metadata.Name, *ingress.Metadata.Namespace, hostname)
				} else {
					log.Info().Msgf("[%v] Ingress %v.%v - Disabling proxying for dns record %v (A)...", initiator, *ingress.Metadata.Name, *ingress.Metadata.Namespace, hostname)
				}

				_, err := cf.UpdateProxySetting(hostname, desiredState.Proxy == "true")
				if err != nil {
					if desiredState.Proxy == "true" {
						log.Error().Err(err).Msgf("[%v] Ingress %v.%v - Enabling proxying for dns record %v (A) failed", initiator, *ingress.Metadata.Name, *ingress.Metadata.Namespace, hostname)
					} else {
						log.Error().Err(err).Msgf("[%v] Ingress %v.%v - Disabling proxying for dns record %v (A) failed", initiator, *ingress.Metadata.Name, *ingress.Metadata.Namespace, hostname)
					}

					return status, err
				}
			}

			// if use origin is disabled, remove the A record for the origin, if state still has a value for OriginRecordHostname
			if desiredState.OriginRecordHostname != "" && (desiredState.UseOriginRecord != "true" || desiredState.OriginRecordHostname == "") {

				log.Info().Msgf("[%v] Ingress %v.%v - Deleting origin dns record %v (A)...", initiator, *ingress.Metadata.Name, *ingress.Metadata.Namespace, desiredState.OriginRecordHostname)

				_, err := cf.DeleteDNSRecord(desiredState.OriginRecordHostname)
				if err != nil {
					log.Error().Err(err).Msgf("[%v] Ingress %v.%v - Deleting origin dns record %v (A) failed", initiator, *ingress.Metadata.Name, *ingress.Metadata.Namespace, desiredState.OriginRecordHostname)
					return status, err
				}
			}

			// if any state property changed make sure to update all
			currentState = desiredState

			log.Info().Msgf("[%v] Ingress %v.%v - Updating ingress because state has changed...", initiator, *ingress.Metadata.Name, *ingress.Metadata.Namespace)

			// serialize state and store it in the annotation
			cloudflareStateByteArray, err := json.Marshal(currentState)
			if err != nil {
				log.Error().Err(err).Msgf("[%v] Ingress %v.%v - Marshalling state failed", initiator, *ingress.Metadata.Name, *ingress.Metadata.Namespace)
				return status, err
			}
			ingress.Metadata.Annotations[annotationCloudflareState] = string(cloudflareStateByteArray)

			// update ingress, because the state annotations have changed
			_, err = client.ExtensionsV1Beta1().UpdateIngress(context.Background(), ingress)
			if err != nil {
				log.Error().Err(err).Msgf("[%v] Ingress %v.%v - Updating ingress state has failed", initiator, *ingress.Metadata.Name, *ingress.Metadata.Namespace)
				return status, err
			}

			status = "succeeded"

			log.Info().Msgf("[%v] Ingress %v.%v - Ingress has been updated successfully...", initiator, *ingress.Metadata.Name, *ingress.Metadata.Namespace)

			return status, nil
		}
	}

	status = "skipped"

	return status, nil
}

func processIngress(cf *Cloudflare, client *k8s.Client, ingress *extensionsv1beta1.Ingress, initiator string) (status string, err error) {

	status = "failed"

	if &ingress != nil && &ingress.Metadata != nil && &ingress.Metadata.Annotations != nil {

		desiredState := getDesiredIngressState(ingress)
		currentState := getCurrentIngressState(ingress)

		status, err = makeIngressChanges(cf, client, ingress, initiator, desiredState, currentState)

		return
	}

	status = "skipped"

	return status, nil
}

func validateHostname(hostname string) bool {
	dnsNameParts := strings.Split(hostname, ".")
	// we need at least a subdomain within a zone
	if len(dnsNameParts) < 2 {
		return false
	}
	// each label needs to be max 63 characters
	for _, label := range dnsNameParts {
		if len(label) > 63 {
			return false
		}
	}
	return true
}
