package api

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/golang/glog"
)

// AMConnection contains values for basic connection to AM
type AMConnection struct {
	BaseURL  string
	User     string
	Password string
	tokenId  string
	Realm    string
}

// AuthNResponse contains values for further AM processes
type AuthNResponse struct {
	TokenID    string `json:"tokenId"`
	SuccessURL string `json:"successUrl"`
}

type agentPayload struct {
	Username        string   `json:"username"`
	Password        string   `json:"userpassword"`
	AgentType       string   `json:"agenttype"`
	Algorithm       string   `json:"com.forgerock.openam.oauth2provider.idTokenSignedResponseAlg"`
	RedirectionUris []string `json:"com.forgerock.openam.oauth2provider.redirectionURIs"`
	Scope           string   `json:"com.forgerock.openam.oauth2provider.scopes"`
	ConsentImplied  string   `json:"isConsentImplied"`
}

// GetAmConnection returns connection to AM server
func GetAmConnection(issoResource *IssoResource) (am *AMConnection, err error) {
	return openAdminConnection(issoResource.oidcUrl, issoResource.oidcUsername, issoResource.oidcPassword)
}

func openAdminConnection(url, username, password string) (am *AMConnection, err error) {
	am = &AMConnection{BaseURL: url, User: username, Password: password}
	err = am.Authenticate()
	return am, err
}

// Authenticate connects to AM server and sets tokenID in AMConnection struct
func (am *AMConnection) Authenticate() error {
	url := am.getRequestURL("/json/authenticate?authIndexType=service&authIndexValue=adminconsoleservice")

	headers := map[string]string{
		"X-OpenAM-Username": FormatAmHeaderString(am.User),
		"X-OpenAM-Password": FormatAmHeaderString(am.Password),
		"Cache-Control":     "no-cache",
		"Content-Type":      "application/json"}

	request, client, err := executeRequest(url, http.MethodPost, headers, nil)
	if err != nil {
		return err
	}

	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("Could not execute request: %s", err)
	}

	defer response.Body.Close()

	body, err := ioutil.ReadAll(response.Body)

	var a AuthNResponse
	err = json.Unmarshal(body, &a)
	if response.StatusCode != 200 {
		return fmt.Errorf("Failed to authenticate %v: %s", response.Status, err)
	}

	am.tokenId = a.TokenID

	return nil
}

// FormatAmHeaderString used to format user and password for OpenAM (ref RFC2047)
func FormatAmHeaderString(headerString string) string {
	return "=?UTF-8?B?" + base64.StdEncoding.EncodeToString([]byte(headerString)) + "?="
}

func (am *AMConnection) getRequestURL(path string) string {
	var strs []string
	strs = append(strs, am.BaseURL)
	strs = append(strs, path)
	return strings.Join(strs, "")
}

func (am *AMConnection) createNewRequest(method, url string, body io.Reader) (*http.Request, error) {
	request, err := http.NewRequest(method, am.getRequestURL(url), body)
	if err != nil {
		return request, fmt.Errorf("Could not create new request, error: %v", err)
	}

	iPlanetCookie := http.Cookie{Name: "iPlanetDirectoryPro", Value: am.tokenId}
	request.AddCookie(&iPlanetCookie)
	request.Header.Set("Content-Type", "application/json")
	return request, nil
}

// AgentExists verifies existence of am agent
func (am *AMConnection) AgentExists(agentName string) bool {
	agentUrl := am.BaseURL + "/json/agents/" + agentName
	headers := map[string]string{"nav-isso": am.tokenId}

	request, client, err := executeRequest(agentUrl, http.MethodGet, headers, nil)
	if err != nil {
		glog.Errorf("Could not execute request: %s", err)
		return false
	}

	response, err := client.Do(request)
	if err != nil {
		glog.Errorf("Could not read response: %s", err)
		return false
	}

	defer response.Body.Close()

	body, _ := ioutil.ReadAll(response.Body)
	var a AuthNResponse

	_ = json.Unmarshal(body, &a)
	if response.StatusCode == 200 {
		glog.Infof(agentName + " already exists")
		return true
	}

	return false
}

// CreateAgent creates am agent on isso server
func (am *AMConnection) CreateAgent(agentName string, redirectionUris []string, issoResource *IssoResource,
	namedConfigurationRequest *NamedConfigurationRequest) error {
	agentUrl := am.BaseURL + "/json/agents/?_action=create"
	headers := map[string]string{
		"nav-isso":     am.tokenId,
		"Content-Type": "application/json"}

	payload, err := json.Marshal(buildAgentPayload(am, agentName, redirectionUris))
	if err != nil {
		return fmt.Errorf("Could not marshal create request: %s", err)
	}

	request, client, err := executeRequest(agentUrl, http.MethodPost, headers, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("Could not execute request to create agent: %s", err)
	}

	response, err := client.Do(request)
	if err != nil {
		glog.Errorf("Could not read response: %s", err)
		return err
	}

	defer response.Body.Close()

	body, _ := ioutil.ReadAll(response.Body)
	var a AuthNResponse

	err = json.Unmarshal(body, &a)
	if response.StatusCode != 200 && response.StatusCode != 201 {
		return fmt.Errorf("%d Agent %s could not be created: %s", response.StatusCode, agentName, err)
	}

	glog.Infof("Agent %s created", agentName)
	return nil
}

// DeleteAgent deletes am agent on isso server
func (am *AMConnection) DeleteAgent(agentName string) error {
	agentUrl := am.BaseURL + "/json/agents/" + agentName
	headers := map[string]string{"nav-isso": am.tokenId}

	request, client, err := executeRequest(agentUrl, http.MethodDelete, headers, nil)
	if err != nil {
		return fmt.Errorf("Could not execute request to delete agent %s: %s", agentName, err)
	}

	response, err := client.Do(request)
	if err != nil {
		glog.Errorf("Could not read response: %s", err)
		return err
	}

	defer response.Body.Close()

	body, _ := ioutil.ReadAll(response.Body)
	var a AuthNResponse

	err = json.Unmarshal(body, &a)
	if response.StatusCode != 200 {
		return fmt.Errorf("Agent %s could not be deleted: %s", agentName, err)
	}
	return nil
}

func executeRequest(url, method string, headers map[string]string, body io.Reader) (*http.Request, *http.Client,
	error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		glog.Errorf("Could not create request: %s", err)
		return nil, nil, err
	}

	for hKey, hValue := range headers {
		req.Header.Add(hKey, hValue)
	}

	client := &http.Client{}

	return req, client, nil
}

func buildAgentPayload(am *AMConnection, agentName string, uris []string) agentPayload {
	agentPayload := agentPayload{
		Username:        agentName,
		Password:        am.Password,
		AgentType:       "OAuth2Client",
		Algorithm:       "RS256",
		Scope:           "[0]=openid",
		ConsentImplied:  "true",
		RedirectionUris: uris,
	}

	return agentPayload
}

// CreateRedirectionUris creates a list of uris for which to configure the openam agent
func CreateRedirectionUris(issoResource *IssoResource, request *NamedConfigurationRequest) []string {
	uriList := []string{}
	defaultServiceDomain := "adeo.no"
	counter := 0

	for _, contextRoot := range request.ContextRoots {
		if contextRoot[:1] != "/" {
			contextRoot = "/" + contextRoot
		}

		if len(issoResource.ingressUrl) > 0 {
			uriList = append(uriList, fmt.Sprintf("[%d]=https://%s%s", counter, issoResource.ingressUrl, contextRoot))
			counter++
		}

		if len(issoResource.loadbalancerUrl) > 0 {
			uriList = append(uriList, fmt.Sprintf("[%d]=https://%s%s", counter, issoResource.loadbalancerUrl,
				contextRoot))
			counter++
		} else {
			uriList = append(uriList, fmt.Sprintf("[%d]=https://app-%s.%s%s", counter, request.Environment,
				defaultServiceDomain, contextRoot))
			counter++
		}
	}

	glog.Infof("Context roots to add: %s", uriList)
	return uriList
}
