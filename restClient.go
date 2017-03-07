package main

import (
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
)

// restClient is the interface to be able to mock http calls to cloudflare api.
type restClient interface {
	Get(string, APIAuthentication) ([]byte, error)
	Post(string, interface{}, APIAuthentication) ([]byte, error)
	Put(string, interface{}, APIAuthentication) ([]byte, error)
	Delete(string, APIAuthentication) ([]byte, error)
}

// realRESTClient is the http client that makes the actual request to cloudflare api.
type realRESTClient struct {
}

// Get calls the cloudflare api for given url and using authentication to get access.
func (r *realRESTClient) Get(cloudflareAPIURL string, authentication APIAuthentication) (body []byte, err error) {
	return core("GET", cloudflareAPIURL, nil, authentication)
}

// Post calls the cloudflare api for given url and using authentication to get access.
func (r *realRESTClient) Post(cloudflareAPIURL string, params interface{}, authentication APIAuthentication) (body []byte, err error) {
	return core("POST", cloudflareAPIURL, params, authentication)
}

// Put calls the cloudflare api for given url and using authentication to get access.
func (r *realRESTClient) Put(cloudflareAPIURL string, params interface{}, authentication APIAuthentication) (body []byte, err error) {
	return core("PUT", cloudflareAPIURL, params, authentication)
}

// Delete calls the cloudflare api for given url and using authentication to get access.
func (r *realRESTClient) Delete(cloudflareAPIURL string, authentication APIAuthentication) (body []byte, err error) {
	return core("DELETE", cloudflareAPIURL, nil, authentication)
}

func core(verb, cloudflareAPIURL string, params interface{}, authentication APIAuthentication) (body []byte, err error) {

	// convert params to json if they're present
	var requestBody io.Reader
	if params != nil {
		data, err := json.Marshal(params)
		if err != nil {
			return body, err
		}
		requestBody = bytes.NewReader(data)
	}

	// create client, in order to add headers
	client := &http.Client{}
	request, err := http.NewRequest(verb, cloudflareAPIURL, requestBody)
	if err != nil {
		return
	}

	// add headers
	request.Header.Add("Content-Type", "application/json")
	request.Header.Add("X-Auth-Key", authentication.Key)
	request.Header.Add("X-Auth-Email", authentication.Email)

	// perform actual request
	response, err := client.Do(request)
	if err != nil {
		return
	}

	defer response.Body.Close()

	body, err = ioutil.ReadAll(response.Body)
	if err != nil {
		return
	}

	return
}
