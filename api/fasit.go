package api

import (
	"encoding/json"
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"io/ioutil"
	"net/http"
	"strconv"
	"regexp"
)

func init() {
	prometheus.MustRegister(httpReqsCounter)
}

type Scope struct {
	EnvironmentClass string `json:"environmentclass"`
	Environment      string `json:"environment,omitempty"`
	Zone             string `json:"zone,omitempty"`
}

type Password struct {
	Ref string `json:"ref"`
}

type Resource struct {
	Id int `json:"id"`
}

type FasitClient struct {
	FasitUrl string
	Username string
	Password string
}

type FasitResource struct {
	Id           int
	Alias        string
	ResourceType string                 `json:"type"`
	Scope        Scope                  `json:"scope"`
	Properties   map[string]string
	Secrets      map[string]map[string]string
	Certificates map[string]interface{} `json:"files"`
}

type ResourceRequest struct {
	Alias        string
	ResourceType string
}

type OpenAmResource struct {
	Hostname string
	Username string
	Password string
}

func (fasit FasitClient) doRequest(r *http.Request) ([]byte, AppError) {
	requestCounter.With(nil).Inc()

	client := &http.Client{}
	resp, err := client.Do(r)

	if err != nil {
		errorCounter.WithLabelValues("contact_fasit").Inc()
		return []byte{}, appError{err, "Error contacting fasit", http.StatusInternalServerError}
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		errorCounter.WithLabelValues("read_body").Inc()
		return []byte{}, appError{err, "Could not read body", http.StatusInternalServerError}
	}

	httpReqsCounter.WithLabelValues(strconv.Itoa(resp.StatusCode), "GET").Inc()
	if resp.StatusCode == 404 {
		errorCounter.WithLabelValues("error_fasit").Inc()
		return []byte{}, appError{nil, "Item not found in Fasit", http.StatusNotFound}
	}

	httpReqsCounter.WithLabelValues(strconv.Itoa(resp.StatusCode), "GET").Inc()
	if resp.StatusCode > 299 {
		errorCounter.WithLabelValues("error_fasit").Inc()
		return []byte{}, appError{nil, "Error contacting Fasit", resp.StatusCode}
	}

	return body, nil

}

func (fasit FasitClient) getOpenAmResource(resourcesRequest ResourceRequest, fasitEnvironment, application, zone string) (OpenAmResource, AppError) {
	fasitResource, fasitErr := getFasitResource(fasit, resourcesRequest, fasitEnvironment, application, zone)
	if fasitErr != nil {
		return OpenAmResource{}, appError{fasitErr, "Could not fetch fasit resource", 500}
	}

	resource, appErr := fasit.mapToOpenAmResource(fasitResource)
	if appErr != nil {
		return OpenAmResource{}, appError{appErr, "Unable to map fasit resource to OpenAM resource", 500}
	}
	return resource, nil
}

func getFasitResource(fasit FasitClient, resourcesRequest ResourceRequest, fasitEnvironment, application, zone string)(FasitResource, AppError) {
	req, err := fasit.buildRequest("GET", "/api/v2/scopedresource", map[string]string{
		"alias":       resourcesRequest.Alias,
		"type":        resourcesRequest.ResourceType,
		"environment": fasitEnvironment,
		"application": application,
		"zone":        zone,
	})

	if err != nil {
		return FasitResource{}, appError{err, "unable to create request", 500}
	}

	body, appErr := fasit.doRequest(req)
	if appErr != nil {
		return FasitResource{}, appErr
	}

	var fasitResource FasitResource

	err = json.Unmarshal(body, &fasitResource)
	if err != nil {
		errorCounter.WithLabelValues("unmarshal_body").Inc()
		return FasitResource{}, appError{err, "Could not unmarshal body", 500}
	}

	return fasitResource, nil
}

func (fasit FasitClient) mapToOpenAmResource(fasitResource FasitResource) (resource OpenAmResource, err error) {
	resource.Hostname = fasitResource.Properties["hostname"]
	resource.Username = fasitResource.Properties["username"]

	if len(fasitResource.Secrets) > 0 {
		secret, err := resolveSecret(fasitResource.Secrets, fasit.Username, fasit.Password)
		if err != nil {
			errorCounter.WithLabelValues("resolve_secret").Inc()
			return OpenAmResource{}, fmt.Errorf("Unable to resolve secret: %s", err)
		}

		resource.Password = secret["password"]
	}
	return resource, nil
}

func (fasit FasitClient) GetFasitEnvironment(environmentName string) (string, error) {
	requestCounter.With(nil).Inc()
	req, err := http.NewRequest("GET", fasit.FasitUrl+"/api/v2/environments/"+environmentName, nil)
	if err != nil {
		return "", fmt.Errorf("Could not create request: %s", err)
	}

	resp, appErr := fasit.doRequest(req)
	if appErr != nil {
		return "", appErr
	}

	type FasitEnvironment struct {
		EnvironmentClass string `json:"environmentclass"`
	}
	var fasitEnvironment FasitEnvironment
	if err := json.Unmarshal(resp, &fasitEnvironment); err != nil {
		return "", fmt.Errorf("Unable to read environmentclass from response: %s", err)
	}

	return fasitEnvironment.EnvironmentClass, nil
}

func resolveSecret(secrets map[string]map[string]string, username string, password string) (map[string]string, error) {
	req, err := http.NewRequest("GET", secrets[getFirstKey(secrets)]["ref"], nil)
	if err != nil {
		return map[string]string{}, err
	}

	req.SetBasicAuth(username, password)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		errorCounter.WithLabelValues("contact_fasit").Inc()
		return map[string]string{}, fmt.Errorf("Error contacting fasit when resolving secret: %s", err)
	}

	httpReqsCounter.WithLabelValues(strconv.Itoa(resp.StatusCode), "GET").Inc()
	if resp.StatusCode > 299 {
		errorCounter.WithLabelValues("error_fasit").Inc()
		return map[string]string{}, fmt.Errorf("Fasit gave errormessage when resolving secret: %s" + strconv.Itoa(resp.StatusCode))
	}

	defer resp.Body.Close()

	body, _ := ioutil.ReadAll(resp.Body)

	return map[string]string{"password": string(body)}, nil
}

func getFirstKey(m map[string]map[string]string) string {
	if len(m) > 0 {
		for key := range m {
			return key
		}
	}
	return ""
}

func (fasit FasitClient) buildRequest(method, path string, queryParams map[string]string) (*http.Request, error) {
	req, err := http.NewRequest(method, fasit.FasitUrl+path, nil)

	if err != nil {
		errorCounter.WithLabelValues("create_request").Inc()
		return nil, fmt.Errorf("could not create request: %s", err)
	}

	q := req.URL.Query()

	for k, v := range queryParams {
		q.Add(k, v)
	}
	req.URL.RawQuery = q.Encode()
	return req, nil
}

func (fasit FasitClient) environmentNameFromNamespaceBuilder(namespace, clustername string) string {
	re := regexp.MustCompile(`^[utqp][0-9]*$`)

	if namespace == "default" || len(namespace) == 0 {
		return clustername
	} else if !re.MatchString(namespace) {
		return namespace + "-" + clustername
	}
	return namespace
}

var httpReqsCounter = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Subsystem: "fasitAdapter",
		Name:      "http_requests_total",
		Help:      "How many HTTP requests processed, partitioned by status code and HTTP method.",
	},
	[]string{"code", "method"})

var requestCounter = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Subsystem: "fasit",
		Name:      "requests",
		Help:      "Incoming requests to fasitadapter",
	},
	[]string{})

var errorCounter = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Subsystem: "fasit",
		Name:      "errors",
		Help:      "Errors occurred in fasitadapter",
	},
	[]string{"type"})
