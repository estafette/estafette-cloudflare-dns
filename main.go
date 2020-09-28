package main

import (
	"encoding/json"
	"math/rand"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/alecthomas/kingpin"
	foundation "github.com/estafette/estafette-foundation"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog/log"
	v1 "k8s.io/api/core/v1"
	networkingv1beta1 "k8s.io/api/networking/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
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
	appgroup  string
	app       string
	version   string
	branch    string
	revision  string
	buildDate string
	goVersion = runtime.Version()
)

var (
	cfAPIKey   = kingpin.Flag("cloudflare-api-key", "The Cloudflare API key.").Envar("CF_API_KEY").Required().String()
	cfAPIEmail = kingpin.Flag("cloudflare-api-email", "The Cloudflare API email address.").Envar("CF_API_EMAIL").Required().String()

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
	kingpin.Parse()

	// init log format from envvar ESTAFETTE_LOG_FORMAT
	foundation.InitLoggingFromEnv(foundation.NewApplicationInfo(appgroup, app, version, branch, revision, buildDate))

	// init /liveness endpoint
	foundation.InitLiveness()

	cf := New(APIAuthentication{Key: *cfAPIKey, Email: *cfAPIEmail})

	// creates the in-cluster config
	kubeClientConfig, err := rest.InClusterConfig()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed getting in-cluster kubernetes config")
	}
	// creates the kubernetes clientset
	kubeClientset, err := kubernetes.NewForConfig(kubeClientConfig)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed creating kubernetes clientset")
	}

	// create the shared informer factory and use the client to connect to Kubernetes API
	factory := informers.NewSharedInformerFactory(kubeClientset, 0)

	// create a channel to stop the shared informers gracefully
	stopper := make(chan struct{})
	defer close(stopper)

	// handle kubernetes API crashes
	defer k8sruntime.HandleCrash()

	foundation.InitMetrics()

	gracefulShutdown, waitGroup := foundation.InitGracefulShutdownHandling()

	// watch services for all namespaces
	watchServices(cf, kubeClientset, factory, waitGroup, stopper)

	// watch ingresses for all namespaces
	watchIngresses(cf, kubeClientset, factory, waitGroup, stopper)

	// loop services and ingresses at large intervals as safety net in case the informers miss something
	go func(waitGroup *sync.WaitGroup) {
		// loop indefinitely
		for {
			// get services for all namespaces
			log.Info().Msg("Listing services for all namespaces...")
			services, err := kubeClientset.CoreV1().Services("").List(metav1.ListOptions{})
			if err != nil {
				log.Error().Err(err).Msg("ListServices call failed")
			}
			log.Info().Msgf("Cluster has %v services", len(services.Items))

			// loop all services
			if services != nil && services.Items != nil {
				for _, service := range services.Items {
					waitGroup.Add(1)
					status, err := processService(cf, kubeClientset, &service, "poller")
					dnsRecordsTotals.With(prometheus.Labels{"namespace": service.Namespace, "status": status, "initiator": "poller", "type": "service"}).Inc()
					waitGroup.Done()

					if err != nil {
						log.Error().Err(err).Msgf("Processing service %v.%v failed", service.Name, service.Namespace)
						continue
					}
				}
			}

			// get ingresses for all namespaces
			log.Info().Msg("Listing ingresses for all namespaces...")
			ingresses, err := kubeClientset.NetworkingV1beta1().Ingresses("").List(metav1.ListOptions{})
			if err != nil {
				log.Error().Err(err).Msg("ListIngresses call failed")
			}
			log.Info().Msgf("Cluster has %v ingresses", len(ingresses.Items))

			// loop all ingresses
			if ingresses != nil && ingresses.Items != nil {
				for _, ingress := range ingresses.Items {

					waitGroup.Add(1)
					status, err := processIngress(cf, kubeClientset, &ingress, "poller")
					dnsRecordsTotals.With(prometheus.Labels{"namespace": ingress.Namespace, "status": status, "initiator": "poller", "type": "ingress"}).Inc()
					waitGroup.Done()

					if err != nil {
						log.Error().Err(err).Msgf("Processing ingress %v.%v failed", ingress.Name, ingress.Namespace)
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

	foundation.HandleGracefulShutdown(gracefulShutdown, waitGroup)
}

func applyJitter(input int) (output int) {

	deviation := int(0.25 * float64(input))

	return input - deviation + r.Intn(2*deviation)
}

func getDesiredServiceState(service *v1.Service) (state CloudflareState) {

	var ok bool

	state.Enabled, ok = service.Annotations[annotationCloudflareDNS]
	if !ok {
		state.Enabled = "false"
	}
	state.Hostnames, ok = service.Annotations[annotationCloudflareHostnames]
	if !ok {
		state.Hostnames = ""
	}
	state.InternalHostnames, ok = service.Annotations[annotationCloudflareInternalHostnames]
	if !ok {
		state.InternalHostnames = ""
	}
	state.Proxy, ok = service.Annotations[annotationCloudflareProxy]
	if !ok {
		state.Proxy = "true"
	}
	state.UseOriginRecord, ok = service.Annotations[annotationCloudflareUseOriginRecord]
	if !ok {
		state.UseOriginRecord = "false"
	}
	state.OriginRecordHostname, ok = service.Annotations[annotationCloudflareOriginRecordHostname]
	if !ok {
		state.OriginRecordHostname = ""
	}

	if service.Spec.Type == "LoadBalancer" && len(service.Status.LoadBalancer.Ingress) > 0 {
		state.IPAddress = service.Status.LoadBalancer.Ingress[0].IP
	}
	if service.Spec.ClusterIP != "" {
		state.InternalIPAddress = service.Spec.ClusterIP
	}

	return
}

func getCurrentServiceState(service *v1.Service) (state CloudflareState) {

	// get state stored in annotations if present or set to empty struct
	cloudflareStateString, ok := service.Annotations[annotationCloudflareState]
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

func makeServiceChanges(cf *Cloudflare, kubeClientset *kubernetes.Clientset, service *v1.Service, initiator string, desiredState, currentState CloudflareState) (status string, err error) {

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

				log.Info().Msgf("[%v] Service %v.%v - Upserting origin dns record %v (A) to ip address %v...", initiator, service.Name, service.Namespace, desiredState.OriginRecordHostname, desiredState.IPAddress)

				_, err := cf.UpsertDNSRecord("A", desiredState.OriginRecordHostname, desiredState.IPAddress, false)
				if err != nil {
					log.Error().Err(err).Msgf("[%v] Service %v.%v - Upserting origin dns record %v (A) to ip address %v failed", initiator, service.Name, service.Namespace, desiredState.OriginRecordHostname, desiredState.IPAddress)
					return status, err
				}
			}

			// loop all hostnames
			hostnames := strings.Split(desiredState.Hostnames, ",")
			for _, hostname := range hostnames {

				// validate hostname, skip if invalid
				if !validateHostname(hostname) {
					log.Error().Err(err).Msgf("[%v] Service %v.%v - Invalid dns record %v, skipping", initiator, service.Name, service.Namespace, hostname)
					continue
				}

				// if use origin is enabled, create a CNAME record pointing to the origin record
				if desiredState.UseOriginRecord == "true" && desiredState.OriginRecordHostname != "" {

					log.Info().Msgf("[%v] Service %v.%v - Upserting dns record %v (CNAME) to value %v...", initiator, service.Name, service.Namespace, hostname, desiredState.OriginRecordHostname)

					_, err := cf.UpsertDNSRecord("CNAME", hostname, desiredState.OriginRecordHostname, desiredState.Proxy == "true")
					if err != nil {
						log.Error().Err(err).Msgf("[%v] Service %v.%v - Upserting dns record %v (CNAME) to value %v failed", initiator, service.Name, service.Namespace, hostname, desiredState.OriginRecordHostname)
						return status, err
					}
				} else {

					log.Info().Msgf("[%v] Service %v.%v - Upserting dns record %v (A) to ip address %v...", initiator, service.Name, service.Namespace, hostname, desiredState.IPAddress)

					_, err := cf.UpsertDNSRecord("A", hostname, desiredState.IPAddress, desiredState.Proxy == "true")
					if err != nil {
						log.Error().Err(err).Msgf("[%v] Service %v.%v - Upserting dns record %v (A) to ip address %v failed", initiator, service.Name, service.Namespace, hostname, desiredState.IPAddress)
						return status, err
					}
				}

				// if proxy is enabled, update it at Cloudflare
				if desiredState.Proxy == "true" {
					log.Info().Msgf("[%v] Service %v.%v - Enabling proxying for dns record %v (A)...", initiator, service.Name, service.Namespace, hostname)
				} else {
					log.Info().Msgf("[%v] Service %v.%v - Disabling proxying for dns record %v (A)...", initiator, service.Name, service.Namespace, hostname)
				}

				_, err := cf.UpdateProxySetting(hostname, desiredState.Proxy == "true")
				if err != nil {
					if desiredState.Proxy == "true" {
						log.Error().Err(err).Msgf("[%v] Service %v.%v - Enabling proxying for dns record %v (A) failed", initiator, service.Name, service.Namespace, hostname)
					} else {
						log.Error().Err(err).Msgf("[%v] Service %v.%v - Disabling proxying for dns record %v (A) failed", initiator, service.Name, service.Namespace, hostname)
					}

					return status, err
				}
			}

			// if use origin is disabled, remove the A record for the origin, if state still has a value for OriginRecordHostname
			if desiredState.OriginRecordHostname != "" && (desiredState.UseOriginRecord != "true" || desiredState.OriginRecordHostname == "") {

				log.Info().Msgf("[%v] Service %v.%v - Deleting origin dns record %v (A)...", initiator, service.Name, service.Namespace, desiredState.OriginRecordHostname)

				_, err := cf.DeleteDNSRecord(desiredState.OriginRecordHostname)
				if err != nil {
					log.Error().Err(err).Msgf("[%v] Service %v.%v - Deleting origin dns record %v (A) failed", initiator, service.Name, service.Namespace, desiredState.OriginRecordHostname)
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

				log.Info().Msgf("[%v] Service %v.%v - Upserting dns record %v (A) to internal ip address %v...", initiator, service.Name, service.Namespace, internalHostname, desiredState.InternalIPAddress)

				_, err := cf.UpsertDNSRecord("A", internalHostname, desiredState.InternalIPAddress, false)
				if err != nil {
					log.Error().Err(err).Msgf("[%v] Service %v.%v - Upserting dns record %v (A) to internal ip address %v failed", initiator, service.Name, service.Namespace, internalHostname, desiredState.InternalIPAddress)
					return status, err
				}
			}
		}
	}

	if hasChanges {

		// if any state property changed make sure to update all
		currentState = desiredState

		log.Info().Msgf("[%v] Service %v.%v - Updating service because state has changed...", initiator, service.Name, service.Namespace)

		// serialize state and store it in the annotation
		cloudflareStateByteArray, err := json.Marshal(currentState)
		if err != nil {
			log.Error().Err(err).Msgf("[%v] Service %v.%v - Marshalling state failed", initiator, service.Name, service.Namespace)
			return status, err
		}
		service.Annotations[annotationCloudflareState] = string(cloudflareStateByteArray)

		// update service, because the state annotations have changed
		service, err = kubeClientset.CoreV1().Services("").Update(service)
		if err != nil {
			log.Error().Err(err).Msgf("[%v] Service %v.%v - Updating service state has failed", initiator, service.Name, service.Namespace)
			return status, err
		}

		status = "succeeded"

		log.Info().Msgf("[%v] Service %v.%v - Service has been updated successfully...", initiator, service.Name, service.Namespace)

		return status, nil
	}

	status = "skipped"

	return status, nil
}

func processService(cf *Cloudflare, kubeClientset *kubernetes.Clientset, service *v1.Service, initiator string) (status string, err error) {

	status = "failed"

	if service != nil {

		desiredState := getDesiredServiceState(service)
		currentState := getCurrentServiceState(service)

		status, err = makeServiceChanges(cf, kubeClientset, service, initiator, desiredState, currentState)

		return
	}

	status = "skipped"

	return status, nil
}

func deleteService(cf *Cloudflare, kubeClientset *kubernetes.Clientset, service *v1.Service, initiator string) (status string, err error) {

	status = "failed"

	if service != nil {

		desiredState := getDesiredServiceState(service)

		dnsRecordType := "A"
		if desiredState.UseOriginRecord == "true" && desiredState.OriginRecordHostname != "" {
			dnsRecordType = "CNAME"
		}

		// loop all hostnames
		hostnames := strings.Split(desiredState.Hostnames, ",")
		for _, hostname := range hostnames {
			log.Info().Msgf("[%v] Service %v.%v - Deleting dns record %v (%v) with ip address %v...", initiator, service.Name, service.Namespace, hostname, dnsRecordType, desiredState.IPAddress)
			_, err = cf.DeleteDNSRecordIfMatching(hostname, dnsRecordType, desiredState.IPAddress)
			if err != nil {
				log.Warn().Err(err).Msgf("[%v] Service %v.%v - Failed deleting dns record %v (%v) with ip address %v...", initiator, service.Name, service.Namespace, hostname, dnsRecordType, desiredState.IPAddress)
			} else {
				status = "deleted"
			}
		}

		return
	}

	status = "skipped"

	return status, nil
}

func getDesiredIngressState(ingress *networkingv1beta1.Ingress) (state CloudflareState) {

	var ok bool

	state.Enabled, ok = ingress.Annotations[annotationCloudflareDNS]
	if !ok {
		state.Enabled = "false"
	}
	state.Hostnames, ok = ingress.Annotations[annotationCloudflareHostnames]
	if !ok {
		state.Hostnames = ""
	}
	state.Proxy, ok = ingress.Annotations[annotationCloudflareProxy]
	if !ok {
		state.Proxy = "true"
	}
	state.UseOriginRecord, ok = ingress.Annotations[annotationCloudflareUseOriginRecord]
	if !ok {
		state.UseOriginRecord = "false"
	}
	state.OriginRecordHostname, ok = ingress.Annotations[annotationCloudflareOriginRecordHostname]
	if !ok {
		state.OriginRecordHostname = ""
	}

	if len(ingress.Status.LoadBalancer.Ingress) > 0 {
		state.IPAddress = ingress.Status.LoadBalancer.Ingress[0].IP
	}

	return
}

func getCurrentIngressState(ingress *networkingv1beta1.Ingress) (state CloudflareState) {

	// get state stored in annotations if present or set to empty struct
	cloudflareStateString, ok := ingress.Annotations[annotationCloudflareState]
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

func makeIngressChanges(cf *Cloudflare, kubeClientset *kubernetes.Clientset, ingress *networkingv1beta1.Ingress, initiator string, desiredState, currentState CloudflareState) (status string, err error) {

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

				log.Info().Msgf("[%v] Ingress %v.%v - Upserting origin dns record %v (A) to ip address %v...", initiator, ingress.Name, ingress.Namespace, desiredState.OriginRecordHostname, desiredState.IPAddress)

				_, err := cf.UpsertDNSRecord("A", desiredState.OriginRecordHostname, desiredState.IPAddress, false)
				if err != nil {
					log.Error().Err(err).Msgf("[%v] Ingress %v.%v - Upserting origin dns record %v (A) to ip address %v failed", initiator, ingress.Name, ingress.Namespace, desiredState.OriginRecordHostname, desiredState.IPAddress)
					return status, err
				}
			}

			// loop all hostnames
			hostnames := strings.Split(desiredState.Hostnames, ",")
			for _, hostname := range hostnames {

				// if use origin is enabled, create a CNAME record pointing to the origin record
				if desiredState.UseOriginRecord == "true" && desiredState.OriginRecordHostname != "" {

					log.Info().Msgf("[%v] Ingress %v.%v - Upserting dns record %v (CNAME) to value %v...", initiator, ingress.Name, ingress.Namespace, hostname, desiredState.OriginRecordHostname)

					_, err := cf.UpsertDNSRecord("CNAME", hostname, desiredState.OriginRecordHostname, desiredState.Proxy == "true")
					if err != nil {
						log.Error().Err(err).Msgf("[%v] Ingress %v.%v - Upserting dns record %v (CNAME) to value %v failed", initiator, ingress.Name, ingress.Namespace, hostname, desiredState.OriginRecordHostname)
						return status, err
					}
				} else {

					log.Info().Msgf("[%v] Ingress %v.%v - Upserting dns record %v (A) to ip address %v...", initiator, ingress.Name, ingress.Namespace, hostname, desiredState.IPAddress)

					_, err := cf.UpsertDNSRecord("A", hostname, desiredState.IPAddress, desiredState.Proxy == "true")
					if err != nil {
						log.Error().Err(err).Msgf("[%v] Ingress %v.%v - Upserting dns record %v (A) to ip address %v failed", initiator, ingress.Name, ingress.Namespace, hostname, desiredState.IPAddress)
						return status, err
					}
				}

				// if proxy is enabled, update it at Cloudflare
				if desiredState.Proxy == "true" {
					log.Info().Msgf("[%v] Ingress %v.%v - Enabling proxying for dns record %v (A)...", initiator, ingress.Name, ingress.Namespace, hostname)
				} else {
					log.Info().Msgf("[%v] Ingress %v.%v - Disabling proxying for dns record %v (A)...", initiator, ingress.Name, ingress.Namespace, hostname)
				}

				_, err := cf.UpdateProxySetting(hostname, desiredState.Proxy == "true")
				if err != nil {
					if desiredState.Proxy == "true" {
						log.Error().Err(err).Msgf("[%v] Ingress %v.%v - Enabling proxying for dns record %v (A) failed", initiator, ingress.Name, ingress.Namespace, hostname)
					} else {
						log.Error().Err(err).Msgf("[%v] Ingress %v.%v - Disabling proxying for dns record %v (A) failed", initiator, ingress.Name, ingress.Namespace, hostname)
					}

					return status, err
				}
			}

			// if use origin is disabled, remove the A record for the origin, if state still has a value for OriginRecordHostname
			if desiredState.OriginRecordHostname != "" && (desiredState.UseOriginRecord != "true" || desiredState.OriginRecordHostname == "") {

				log.Info().Msgf("[%v] Ingress %v.%v - Deleting origin dns record %v (A)...", initiator, ingress.Name, ingress.Namespace, desiredState.OriginRecordHostname)

				_, err := cf.DeleteDNSRecord(desiredState.OriginRecordHostname)
				if err != nil {
					log.Error().Err(err).Msgf("[%v] Ingress %v.%v - Deleting origin dns record %v (A) failed", initiator, ingress.Name, ingress.Namespace, desiredState.OriginRecordHostname)
					return status, err
				}
			}

			// if any state property changed make sure to update all
			currentState = desiredState

			log.Info().Msgf("[%v] Ingress %v.%v - Updating ingress because state has changed...", initiator, ingress.Name, ingress.Namespace)

			// serialize state and store it in the annotation
			cloudflareStateByteArray, err := json.Marshal(currentState)
			if err != nil {
				log.Error().Err(err).Msgf("[%v] Ingress %v.%v - Marshalling state failed", initiator, ingress.Name, ingress.Namespace)
				return status, err
			}
			ingress.Annotations[annotationCloudflareState] = string(cloudflareStateByteArray)

			// update ingress, because the state annotations have changed
			_, err = kubeClientset.NetworkingV1beta1().Ingresses(ingress.Namespace).Update(ingress)
			if err != nil {
				log.Error().Err(err).Msgf("[%v] Ingress %v.%v - Updating ingress state has failed", initiator, ingress.Name, ingress.Namespace)
				return status, err
			}

			status = "succeeded"

			log.Info().Msgf("[%v] Ingress %v.%v - Ingress has been updated successfully...", initiator, ingress.Name, ingress.Namespace)

			return status, nil
		}
	}

	status = "skipped"

	return status, nil
}

func processIngress(cf *Cloudflare, kubeClientset *kubernetes.Clientset, ingress *networkingv1beta1.Ingress, initiator string) (status string, err error) {

	status = "failed"

	if ingress != nil {

		desiredState := getDesiredIngressState(ingress)
		currentState := getCurrentIngressState(ingress)

		status, err = makeIngressChanges(cf, kubeClientset, ingress, initiator, desiredState, currentState)

		return
	}

	status = "skipped"

	return status, nil
}

func deleteIngress(cf *Cloudflare, kubeClientset *kubernetes.Clientset, ingress *networkingv1beta1.Ingress, initiator string) (status string, err error) {

	status = "failed"

	if ingress != nil {

		desiredState := getDesiredIngressState(ingress)

		dnsRecordType := "A"
		if desiredState.UseOriginRecord == "true" && desiredState.OriginRecordHostname != "" {
			dnsRecordType = "CNAME"
		}

		// loop all hostnames
		hostnames := strings.Split(desiredState.Hostnames, ",")
		for _, hostname := range hostnames {
			log.Info().Msgf("[%v] Ingress %v.%v - Deleting dns record %v (%v) with ip address %v...", initiator, ingress.Name, ingress.Namespace, hostname, dnsRecordType, desiredState.IPAddress)
			_, err = cf.DeleteDNSRecordIfMatching(hostname, dnsRecordType, desiredState.IPAddress)
			if err != nil {
				log.Warn().Err(err).Msgf("[%v] Ingress %v.%v - Failed deleting dns record %v (%v) with ip address %v...", initiator, ingress.Name, ingress.Namespace, hostname, dnsRecordType, desiredState.IPAddress)
			} else {
				status = "deleted"
			}
		}

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

func watchServices(cf *Cloudflare, kubeClientset *kubernetes.Clientset, factory informers.SharedInformerFactory, waitGroup *sync.WaitGroup, stopper chan struct{}) {
	servicesInformer := factory.Core().V1().Services().Informer()

	servicesInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			service, ok := obj.(*v1.Service)
			if !ok {
				log.Warn().Msg("Watcher for services returns event object of incorrect type")
				return
			}

			waitGroup.Add(1)
			status, err := processService(cf, kubeClientset, service, "watcher:added")
			dnsRecordsTotals.With(prometheus.Labels{"namespace": service.Namespace, "status": status, "initiator": "watcher", "type": "service"}).Inc()
			waitGroup.Done()

			if err != nil {
				log.Error().Err(err).Msgf("Processing service %v.%v failed", service.Name, service.Namespace)
			}
		},
		UpdateFunc: func(oldObj interface{}, newObj interface{}) {

			service, ok := newObj.(*v1.Service)
			if !ok {
				log.Warn().Msg("Watcher for services returns event object of incorrect type")
				return
			}

			waitGroup.Add(1)
			status, err := processService(cf, kubeClientset, service, "watcher:modified")
			dnsRecordsTotals.With(prometheus.Labels{"namespace": service.Namespace, "status": status, "initiator": "watcher", "type": "service"}).Inc()
			waitGroup.Done()

			if err != nil {
				log.Error().Err(err).Msgf("Processing service %v.%v failed", service.Name, service.Namespace)
			}
		},
		DeleteFunc: func(obj interface{}) {

			service, ok := obj.(*v1.Service)
			if !ok {
				log.Warn().Msg("Watcher for services returns event object of incorrect type")
				return
			}

			waitGroup.Add(1)
			status, err := deleteService(cf, kubeClientset, service, "watcher:deleted")
			dnsRecordsTotals.With(prometheus.Labels{"namespace": service.Namespace, "status": status, "initiator": "watcher", "type": "service"}).Inc()
			waitGroup.Done()

			if err != nil {
				log.Error().Err(err).Msgf("Deleting service %v.%v failed", service.Name, service.Namespace)
			}
		},
	})

	go servicesInformer.Run(stopper)
}

func watchIngresses(cf *Cloudflare, kubeClientset *kubernetes.Clientset, factory informers.SharedInformerFactory, waitGroup *sync.WaitGroup, stopper chan struct{}) {
	ingressesInformer := factory.Networking().V1beta1().Ingresses().Informer()

	ingressesInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			ingress, ok := obj.(*networkingv1beta1.Ingress)
			if !ok {
				log.Warn().Msg("Watcher for ingresses returns event object of incorrect type")
				return
			}

			waitGroup.Add(1)
			status, err := processIngress(cf, kubeClientset, ingress, "watcher:added")
			dnsRecordsTotals.With(prometheus.Labels{"namespace": ingress.Namespace, "status": status, "initiator": "watcher", "type": "ingress"}).Inc()
			waitGroup.Done()

			if err != nil {
				log.Error().Err(err).Msgf("Processing ingress %v.%v failed", ingress.Name, ingress.Namespace)
			}
		},
		UpdateFunc: func(oldObj interface{}, newObj interface{}) {

			ingress, ok := newObj.(*networkingv1beta1.Ingress)
			if !ok {
				log.Warn().Msg("Watcher for ingresses returns event object of incorrect type")
				return
			}

			waitGroup.Add(1)
			status, err := processIngress(cf, kubeClientset, ingress, "watcher:modified")
			dnsRecordsTotals.With(prometheus.Labels{"namespace": ingress.Namespace, "status": status, "initiator": "watcher", "type": "ingress"}).Inc()
			waitGroup.Done()

			if err != nil {
				log.Error().Err(err).Msgf("Processing ingress %v.%v failed", ingress.Name, ingress.Namespace)
			}

		},
		DeleteFunc: func(obj interface{}) {

			ingress, ok := obj.(*networkingv1beta1.Ingress)
			if !ok {
				log.Warn().Msg("Watcher for ingresses returns event object of incorrect type")
				return
			}

			waitGroup.Add(1)
			status, err := deleteIngress(cf, kubeClientset, ingress, "watcher:delete")
			dnsRecordsTotals.With(prometheus.Labels{"namespace": ingress.Namespace, "status": status, "initiator": "watcher", "type": "ingress"}).Inc()
			waitGroup.Done()

			if err != nil {
				log.Error().Err(err).Msgf("Deleting ingress %v.%v failed", ingress.Name, ingress.Namespace)
			}
		},
	})

	go ingressesInformer.Run(stopper)
}
