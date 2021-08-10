package api

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"

	"bytes"

	"github.com/golang/glog"
	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	prometheus.MustRegister(httpReqsCounter)
	prometheus.MustRegister(requestCounter)
	prometheus.MustRegister(errorCounter)
}

type scope struct {
	EnvironmentClass string `json:"environmentclass"`
	Environment      string `json:"environment,omitempty"`
	Zone             string `json:"zone,omitempty"`
	Application      string `json:"application,omitempty"`
}

// Password contains fasit reference to the password
type Password struct {
	Ref string `json:"ref"`
}

// Resource contains resource id as set in fasit
type Resource struct {
	ID int `json:"id"`
}

// FasitClient contains fasit connection details
type FasitClient struct {
	FasitURL string
	Username string
	Password string
}

// FasitResource contains resource information from fasit
type FasitResource struct {
	ID           int
	Alias        string                       `json:"alias"`
	ResourceType string                       `json:"type"`
	Scope        scope                        `json:"scope"`
	Properties   map[string]string            `json:"properties"`
	Secrets      map[string]map[string]string `json:"secrets"`
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
	oidcURL           string
	oidcUsername      string
	oidcPassword      string
	oidcAgentPassword string
	IssoIssuerURL     string
	IssoJwksURL       string
	loadbalancerURL   string
	ingressURLs       []string
	contextRoots      []string
	nodes             []string
	createLocalhost   bool
}

const (
	openidconnectalias      = "OpenIdConnect"
	openidconnectagentalias = "OpenIdConnectAgent"
	ResourceTypeOIDC        = "OpenIdConnect"
	ResourceTypeOpenAM      = "OpenAM"
)

func (fasit FasitClient) CreateFasitResourceForOpenIDConnect(issoResource IssoResource, request *NamedConfigurationRequest, zone string) (FasitResource, *AppError) {
	environmentClass, err := fasit.getFasitEnvironment(request.Environment)
	if err != nil {
		glog.Errorf("Failed to retrieve EnvironmentClass from Fasit: %s", err)
		return FasitResource{}, &AppError{err, "Failed to retreive EnvironmentClass from Fasit", http.StatusInternalServerError}
	}

	resource := FasitResource{
		Alias:        fmt.Sprintf("%s-oidc", request.Application),
		ResourceType: ResourceTypeOIDC,
		Scope: scope{
			Application:      request.Application,
			EnvironmentClass: environmentClass,
			Environment:      request.Environment,
			Zone:             zone,
		},
		Properties: map[string]string{
			"agentName": request.Application + "-" + request.Environment,
			"hostUrl":   issoResource.oidcURL,
			"issuerUrl": issoResource.IssoIssuerURL,
			"jwksUrl":   issoResource.IssoJwksURL,
		},
		Secrets: map[string]map[string]string{
			"password": {
				"value": issoResource.oidcAgentPassword,
			},
		},
	}

	return resource, nil
}

func (fasit FasitClient) doRequest(r *http.Request) ([]byte, *AppError) {
	requestCounter.With(nil).Inc()

	client := &http.Client{}
	resp, err := client.Do(r)

	if err != nil {
		errorCounter.WithLabelValues("contact_fasit").Inc()
		return []byte{}, &AppError{err, "Error contacting fasit", http.StatusInternalServerError}
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		errorCounter.WithLabelValues("read_body").Inc()
		return []byte{}, &AppError{err, "Could not read body", http.StatusInternalServerError}
	}

	httpReqsCounter.WithLabelValues(strconv.Itoa(resp.StatusCode), "GET").Inc()
	if resp.StatusCode == 404 {
		errorCounter.WithLabelValues("error_fasit").Inc()
		return []byte{}, &AppError{nil, "Item not found in Fasit: " + r.URL.Scheme + "://" + r.URL.Host + r.URL.RequestURI(), http.StatusNotFound}
	}

	httpReqsCounter.WithLabelValues(strconv.Itoa(resp.StatusCode), "GET").Inc()
	if resp.StatusCode > 299 {
		errorCounter.WithLabelValues("error_fasit").Inc()
		return []byte{}, &AppError{nil, "Error calling Fasit url " + r.URL.Scheme + "://" + r.URL.Host + r.URL.RequestURI(), resp.StatusCode}
	}

	return body, nil

}

// GetIssoResource fetches necessary ISSO and OIDC resources from fasit
func (fasit FasitClient) GetIssoResource(request *NamedConfigurationRequest, zone string) (IssoResource, *AppError) {
	fasitEnvironment := request.Environment
	application := request.Application

	oidcURLResourceRequest := ResourceRequest{openidconnectalias, "BaseUrl"}
	oidcURLResource, fasitErr := getFasitResource(fasit, oidcURLResourceRequest, fasitEnvironment, application, zone)
	if fasitErr != nil {
		return IssoResource{}, fasitErr
	}

	oidcResourceRequest := ResourceRequest{openidconnectalias, "Credential"}
	oidcUserResource, fasitErr := getFasitResource(fasit, oidcResourceRequest, fasitEnvironment, application, zone)
	if fasitErr != nil {
		return IssoResource{}, fasitErr
	}

	oidcAgentResourceRequest := ResourceRequest{openidconnectagentalias, "Credential"}
	oidcAgentResource, fasitErr := getFasitResource(fasit, oidcAgentResourceRequest, fasitEnvironment, application, zone)
	if fasitErr != nil {
		return IssoResource{}, fasitErr
	}

	loadbalancerResourceRequest := ResourceRequest{"loadbalancer:" + application, "BaseUrl"}
	loadbalancerResource, _ := getFasitResource(fasit, loadbalancerResourceRequest, fasitEnvironment,
		application, zone)

	ingressUrls, err := fasit.GetIngressURL(request, zone)
	if err != nil {
		return IssoResource{}, &AppError{err, "Could not fetch ingress url for application", 404}
	}

	resource, appErr := fasit.mapToIssoResource(oidcURLResource, oidcUserResource, oidcAgentResource,
		loadbalancerResource, ingressUrls)
	if appErr != nil {
		return IssoResource{}, appErr
	}

	return resource, nil
}

// GetOpenAmResource fetches necessary OpenAM resources from fasit
func (fasit FasitClient) GetOpenAmResource(resourcesRequest ResourceRequest, fasitEnvironment, application, zone string) (OpenAmResource, *AppError) {
	fasitResource, fasitErr := getFasitResource(fasit, resourcesRequest, fasitEnvironment, application, zone)
	if fasitErr != nil {
		return OpenAmResource{}, fasitErr
	}

	resource, appErr := fasit.mapToOpenAmResource(fasitResource)
	if appErr != nil {
		return OpenAmResource{}, appErr
	}
	return resource, nil
}

func getFasitResource(fasit FasitClient, resourcesRequest ResourceRequest, fasitEnvironment, application, zone string) (FasitResource, *AppError) {
	req, err := fasit.buildRequestWithQueryParams("GET", "/api/v2/scopedresource", map[string]string{
		"alias":       resourcesRequest.Alias,
		"type":        resourcesRequest.ResourceType,
		"environment": fasitEnvironment,
		"application": application,
		"zone":        zone,
	})

	if err != nil {
		return FasitResource{}, &AppError{err, "Unable to create request", 500}
	}

	body, appErr := fasit.doRequest(req)
	if appErr != nil {
		return FasitResource{}, appErr
	}

	var fasitResource FasitResource

	err = json.Unmarshal(body, &fasitResource)
	if err != nil {
		errorCounter.WithLabelValues("unmarshal_body").Inc()
		return FasitResource{}, &AppError{err, "Could not unmarshal body", 500}
	}

	return fasitResource, nil
}

func (fasit FasitClient) mapToIssoResource(oidcURLResource FasitResource, oidcUserResource FasitResource,
	oidcAgentResource FasitResource, loadbalancerResource FasitResource, ingressUrls []string) (resource IssoResource,
	appErr *AppError) {
	resource.oidcURL = oidcURLResource.Properties["url"]
	issoURL, err := InsertPortNumber(oidcURLResource.Properties["url"]+"/oauth2", 443)
	if err != nil {
		return IssoResource{}, &AppError{err, "Could not parse url", http.StatusInternalServerError}
	}
	resource.IssoIssuerURL = issoURL
	resource.IssoJwksURL = oidcURLResource.Properties["url"] + "/oauth2/connect/jwk_uri"
	resource.oidcUsername = oidcUserResource.Properties["username"]

	if len(oidcUserResource.Secrets) > 0 {
		secret, err := resolveSecret(oidcUserResource.Secrets, fasit.Username, fasit.Password)
		if err != nil {
			errorCounter.WithLabelValues("resolve_secret").Inc()
			return IssoResource{}, err
		}

		resource.oidcPassword = secret["password"]
	}
	if len(oidcAgentResource.Secrets) > 0 {
		secret, err := resolveSecret(oidcAgentResource.Secrets, fasit.Username, fasit.Password)
		if err != nil {
			errorCounter.WithLabelValues("resolve_secret").Inc()
			return IssoResource{}, err
		}

		resource.oidcAgentPassword = secret["password"]
	}

	resource.loadbalancerURL = loadbalancerResource.Properties["url"]
	resource.ingressURLs = ingressUrls

	return resource, nil
}

func (fasit FasitClient) mapToOpenAmResource(fasitResource FasitResource) (resource OpenAmResource, appErr *AppError) {
	resource.Hostname = fasitResource.Properties["hostname"]
	resource.Username = fasitResource.Properties["username"]

	if len(fasitResource.Secrets) > 0 {
		secret, err := resolveSecret(fasitResource.Secrets, fasit.Username, fasit.Password)
		if err != nil {
			errorCounter.WithLabelValues("resolve_secret").Inc()
			return OpenAmResource{}, err
		}

		resource.Password = secret["password"]
	}
	return resource, nil
}

// GetFasitApplication returns nil if application exists in Fasit
func (fasit FasitClient) GetFasitApplication(application string) *AppError {
	requestCounter.With(nil).Inc()
	req, err := http.NewRequest("GET", fasit.FasitURL+"/api/v2/applications/"+application, nil)
	if err != nil {
		return &AppError{err, "Could not create request", http.StatusInternalServerError}
	}

	_, appErr := fasit.doRequest(req)
	if appErr != nil {
		return &AppError{fmt.Errorf("could not find application %s in Fasit", application), "Application does not " +
			"exist", http.StatusNotFound}
	}

	return nil
}

// GetFasitEnvironment converts Fasit environment name to environment class
func (fasit FasitClient) GetFasitEnvironment(environmentName string) (string, *AppError) {
	requestCounter.With(nil).Inc()
	req, err := http.NewRequest("GET", fasit.FasitURL+"/api/v2/environments/"+environmentName, nil)
	if err != nil {
		return "", &AppError{err, "Could not create request", http.StatusInternalServerError}
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
		return "", &AppError{err, "Could not read environment from response", http.StatusInternalServerError}
	}

	return fasitEnvironment.EnvironmentClass, nil
}

func resolveSecret(secrets map[string]map[string]string, username string, password string) (map[string]string, *AppError) {
	req, err := http.NewRequest("GET", secrets[getFirstKey(secrets)]["ref"], nil)
	if err != nil {
		return map[string]string{}, &AppError{err, "Could not create request to resolve secret", http.StatusBadRequest}
	}

	req.SetBasicAuth(username, password)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		errorCounter.WithLabelValues("contact_fasit").Inc()
		return map[string]string{}, &AppError{err, "Could not contact fasit", http.StatusBadRequest}
	}

	httpReqsCounter.WithLabelValues(strconv.Itoa(resp.StatusCode), "GET").Inc()
	if 401 == resp.StatusCode {
		errorCounter.WithLabelValues("error_fasit").Inc()
		return map[string]string{}, &AppError{err, "Authorization failed when contacting fasit", http.StatusUnauthorized}
	} else if resp.StatusCode > 299 {
		errorCounter.WithLabelValues("error_fasit").Inc()
		return map[string]string{}, &AppError{err, "Fasit error when resolving secret", resp.StatusCode}
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

func (fasit FasitClient) UpdateFasitResource(resource FasitResource, request *NamedConfigurationRequest) *AppError {
	payload, err := json.Marshal(resource)
	if err != nil {
		errorCounter.WithLabelValues("marshal_body").Inc()
		return &AppError{err, "Could not marshal openIdConnect resource", 500}
	}

	req, err := fasit.buildRequestWithPayload("PUT", fmt.Sprintf("/api/v2/resources/%d", resource.ID), payload, request)
	if err != nil {
		errorCounter.WithLabelValues("marshal_body").Inc()
		return &AppError{err, "Error when building request with payload", 500}
	}

	_, appErr := fasit.doRequest(req)
	if appErr != nil {
		return appErr
	}

	return nil
}

func (fasit FasitClient) PostFasitResource(resource FasitResource, request *NamedConfigurationRequest) *AppError {
	payload, err := json.Marshal(resource)
	if err != nil {
		return &AppError{err, "Could not marshal openIdConnect resource", 500}
	}

	req, err := fasit.buildRequestWithPayload("POST", "/api/v2/resources", payload, request)
	if err != nil {
		return &AppError{err, "Error when building request with payload", 500}
	}

	_, appErr := fasit.doRequest(req)
	if appErr != nil {
		return appErr
	}

	return nil
}

func (fasit FasitClient) buildRequestWithQueryParams(method, path string, queryParams map[string]string) (*http.Request, error) {
	req, err := http.NewRequest(method, fasit.FasitURL+path, nil)

	if err != nil {
		errorCounter.WithLabelValues("create_request").Inc()
		return nil, err
	}

	q := req.URL.Query()

	for k, v := range queryParams {
		q.Add(k, v)
	}
	req.URL.RawQuery = q.Encode()
	return req, nil
}

func (fasit FasitClient) buildRequestWithPayload(method, path string, payload []byte, request *NamedConfigurationRequest) (*http.Request,
	error) {
	req, err := http.NewRequest(method, fasit.FasitURL+path, bytes.NewBuffer(payload))
	req.SetBasicAuth(request.Username, request.Password)
	req.Header.Set("Content-Type", "application/json")
	if err != nil {
		errorCounter.WithLabelValues("create_request").Inc()
		return nil, err
	}

	return req, nil
}

func InsertPortNumber(originalUrl string, port int) (string, error) {
	u, err := url.Parse(originalUrl)
	if err != nil {
		fmt.Println("Port: " + u.Port())
		return originalUrl, err
	}
	if urlContainsPortNumber(originalUrl) {
		return originalUrl, nil
	}

	return u.Scheme + "://" + u.Host + ":" + strconv.Itoa(port) + u.Path, nil
}

func urlContainsPortNumber(urlString string) bool {
	u, err := url.Parse(urlString)
	if err != nil || len(u.Port()) > 0 {
		return true
	}

	return false
}


func (fasit FasitClient) getFasitEnvironment(environmentName string) (string, error) {
	requestCounter.With(nil).Inc()
	req, err := http.NewRequest("GET", fasit.FasitURL+"/api/v2/environments/"+environmentName, nil)
	if err != nil {
		return "", err
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
		return "", err
	}

	return fasitEnvironment.EnvironmentClass, nil
}

// GetIngressURL creates ingress urls from environment class and zone
func (fasit FasitClient) GetIngressURL(request *NamedConfigurationRequest, zone string) ([]string, error) {
	environmentClass, err := fasit.getFasitEnvironment(request.Environment)
	if err != nil {
		return []string{}, err
	}

	domain, newDomain, naisDeviceDomain := GetDomainsFromZoneAndEnvironmentClass(environmentClass, zone)
	ingressUrls := []string{
		fmt.Sprintf("%s.%s", request.Application, domain),
		fmt.Sprintf("%s-%s.%s", request.Application, request.Environment, domain),
		fmt.Sprintf("%s.%s", request.Application, newDomain),
		fmt.Sprintf("%s-%s.%s", request.Application, request.Environment, newDomain),
	}

	ingressUrls = append(
		ingressUrls,
		fmt.Sprintf("%s.%s", request.Application, naisDeviceDomain),
		fmt.Sprintf("%s-%s.%s", request.Application, request.Environment, naisDeviceDomain),
	)

	return ingressUrls, nil
}

// GetDomainFromZoneAndEnvironmentClass returns domain string
func GetDomainsFromZoneAndEnvironmentClass(environmentClass, zone string) (string, string, string) {
	// Valid for dev-fss
	domain := "nais.preprod.local"
	newDomain := "dev-fss.nais.io"
	naisDeviceDomain := "dev.intern.nav.no"

	if ZoneFss == zone && environmentClass == "p" {
		domain = "nais.adeo.no"
		newDomain = "prod-fss.nais.io"
		naisDeviceDomain = "intern.nav.no"
	}

	if ZoneSbs == zone {
		switch environmentClass {
		case "p":
			domain = "nais.oera.no"
			newDomain = "prod-sbs.nais.io"
			naisDeviceDomain = "nav.no"
		default:
			domain = "nais.oera-q.local"
			newDomain = "dev-sbs.nais.io"
			naisDeviceDomain = "dev.nav.no"
		}
	}

	return domain, newDomain, naisDeviceDomain
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
