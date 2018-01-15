package api

import (
	"encoding/json"
	"fmt"
	"github.com/golang/glog"
	"github.com/prometheus/client_golang/prometheus"
	"io/ioutil"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
)

func init() {
	prometheus.MustRegister(httpReqsCounter)
}

type scope struct {
	EnvironmentClass string `json:"environmentclass"`
	Environment      string `json:"environment,omitempty"`
	Zone             string `json:"zone,omitempty"`
}

// Password contains fasit reference to the password
type Password struct {
	Ref string `json:"ref"`
}

// Resource contains resource id as set in fasit
type Resource struct {
	Id int `json:"id"`
}

// FasitClient contains fasit connection details
type FasitClient struct {
	FasitUrl string
	Username string
	Password string
}

// FasitResource contains resource information from fasit
type FasitResource struct {
	Id           int
	Alias        string
	ResourceType string `json:"type"`
	Scope        scope  `json:"scope"`
	Properties   map[string]string
	Secrets      map[string]map[string]string
	Certificates map[string]interface{} `json:"files"`
}

// ResourceRequest contains the alias and resource type for the fasit resource
type ResourceRequest struct {
	Alias        string
	ResourceType string
}

// OpenAmResource contains information about the AM server as set in fasit
type OpenAmResource struct {
	Hostname string
	Username string
	Password string
}

// IssoResource contains information about the OIDC server as set in fasit
type IssoResource struct {
	oidcUrl           string
	oidcUsername      string
	oidcPassword      string
	oidcAgentPassword string
	IssoIssuerUrl     string
	IssoJwksUrl       string
}

const (
	OPENIDCONNECTALIAS      = "OpenIdConnect"
	OPENIDCONNECTAGENTALIAS = "OpenIdConnectAgent"
)

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

// GetIssoResource fetches necessary ISSO and OIDC resources from fasit
func (fasit FasitClient) GetIssoResource(fasitEnvironment, application, zone string) (IssoResource, AppError) {
	oidcUrlResourceRequest := ResourceRequest{OPENIDCONNECTALIAS, "BaseUrl"}
	oidcUrlResource, fasitErr := getFasitResource(fasit, oidcUrlResourceRequest, fasitEnvironment, application, zone)
	if fasitErr != nil {
		return IssoResource{}, appError{fasitErr, "Could not fetch fasit resource isso-rp-use", 404}
	}

	oidcResourceRequest := ResourceRequest{OPENIDCONNECTALIAS, "Credential"}
	oidcUserResource, fasitErr := getFasitResource(fasit, oidcResourceRequest, fasitEnvironment, application, zone)
	if fasitErr != nil {
		return IssoResource{}, appError{fasitErr, "Could not fetch fasit resource isso-rp-use", 404}
	}

	oidcAgentResourceRequest := ResourceRequest{OPENIDCONNECTAGENTALIAS, "Credential"}
	oidcAgentResource, fasitErr := getFasitResource(fasit, oidcAgentResourceRequest, fasitEnvironment, application, zone)
	if fasitErr != nil {
		return IssoResource{}, appError{fasitErr, "Could not fetch fasit resource isso-rp-use", 404}
	}

	resource, appErr := fasit.mapToIssoResource(oidcUrlResource, oidcUserResource, oidcAgentResource)
	if appErr != nil {
		return IssoResource{}, appError{appErr, "Unable to map fasit resources to Isso resource", 500}
	}
	return resource, nil
}

// GetOpenAmResource fetches necessary OpenAM resources from fasit
func (fasit FasitClient) GetOpenAmResource(resourcesRequest ResourceRequest, fasitEnvironment, application, zone string) (OpenAmResource, AppError) {
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

func getFasitResource(fasit FasitClient, resourcesRequest ResourceRequest, fasitEnvironment, application, zone string) (FasitResource, AppError) {
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

func (fasit FasitClient) mapToIssoResource(oidcUrlResource FasitResource, oidcUserResource FasitResource, oidcAgentResource FasitResource) (resource IssoResource, err error) {
	resource.oidcUrl = oidcUrlResource.Properties["url"]
	issoUrl, err := insertPortNumber(oidcUrlResource.Properties["url"]+"/oauth2", 443)
	if err != nil {
		glog.Errorf("Could not parse url %s", err)
	}
	resource.IssoIssuerUrl = issoUrl
	resource.IssoJwksUrl = oidcUrlResource.Properties["url"] + "/oauth2/connect/jwk_uri"
	resource.oidcUsername = oidcUserResource.Properties["username"]

	if len(oidcUserResource.Secrets) > 0 {
		secret, err := resolveSecret(oidcUserResource.Secrets, fasit.Username, fasit.Password)
		if err != nil {
			errorCounter.WithLabelValues("resolve_secret").Inc()
			return IssoResource{}, fmt.Errorf("Unable to resolve secret: %s", err)
		}

		resource.oidcPassword = secret["password"]
	}
	if len(oidcAgentResource.Secrets) > 0 {
		secret, err := resolveSecret(oidcAgentResource.Secrets, fasit.Username, fasit.Password)
		if err != nil {
			errorCounter.WithLabelValues("resolve_secret").Inc()
			return IssoResource{}, fmt.Errorf("Unable to resolve secret: %s", err)
		}

		resource.oidcAgentPassword = secret["password"]
	}

	return resource, nil
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

// GetFasitEnvironment returns fasit environment string from fasit REST endpoint
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

func insertPortNumber(urlWithoutPort string, port int) (string, error) {
	u, err := url.Parse(urlWithoutPort)
	if err != nil {
		return urlWithoutPort, err
	}
	return u.Scheme + "://" + u.Host + ":" + strconv.Itoa(port) + u.Path, nil
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
