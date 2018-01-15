package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/golang/glog"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
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
func GetAmConnection(issoResource IssoResource) (am *AMConnection, err error) {
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
		"X-Openam-Username": am.User,
		"X-Openam-Password": am.Password,
		"Cache-Control":     "no-cache"}

	response, err := executeRequest(url, http.MethodPost, headers, nil)
	if err != nil {
		return err
	}

	body, _ := ioutil.ReadAll(response.Body)
	var a AuthNResponse

	err = json.Unmarshal(body, &a)
	if response.StatusCode != 200 {
		return fmt.Errorf("Failed to authenticate %v: %s", response.Status, err)
	}

	am.tokenId = a.TokenID

	return nil
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
	response, err := executeRequest(agentUrl, http.MethodGet, headers, nil)
	if err != nil {
		glog.Errorf("Could not execute request: %s", err)
		return false
	}

	body, _ := ioutil.ReadAll(response.Body)
	var a AuthNResponse

	_ = json.Unmarshal(body, &a)
	if response.StatusCode == 200 {
		glog.Infof(agentName + " already exists")
		return true
	}
	glog.Infof(agentName + " does not exist")
	return false
}

// CreateAgent creates am agent on isso server
func (am *AMConnection) CreateAgent(agentName string, redirectionUris []string) (bool, error) {
	agentUrl := am.BaseURL + "/json/agents/?_action=create"
	headers := map[string]string{"nav-isso": am.tokenId}
	payload, err := json.Marshal(buildAgentPayload(am, agentName, redirectionUris))
	response, err := executeRequest(agentUrl, http.MethodPost, headers, bytes.NewReader(payload))
	if err != nil {
		return false, fmt.Errorf("Could not execute request to create agent: %s", err)
	}

	body, _ := ioutil.ReadAll(response.Body)
	var a AuthNResponse

	_ = json.Unmarshal(body, &a)
	if response.StatusCode != 200 {
		return false, fmt.Errorf("Agent %s could not be created: %s", agentName, err)
	}
	glog.Infof("Agent %s created", agentName)
	return true, nil
}

// DeleteAgent deletes am agent on isso server
func (am *AMConnection) DeleteAgent(agentName string) error {
	agentUrl := am.BaseURL + "/json/agents/" + agentName
	headers := map[string]string{"nav-isso": am.tokenId}
	response, err := executeRequest(agentUrl, http.MethodDelete, headers, nil)

	body, _ := ioutil.ReadAll(response.Body)
	var a AuthNResponse

	_ = json.Unmarshal(body, &a)
	if response.StatusCode != 200 {
		return fmt.Errorf("Agent %s could not be deleted: %s", agentName, err)
	}
	return nil
}

func executeRequest(url, method string, headers map[string]string, body io.Reader) (*http.Response,
	error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		glog.Errorf("Could not create request: %s", err)
	}

	for hKey, hValue := range headers {
		req.Header.Set(hKey, fmt.Sprintf("%q", hValue))
	}

	client := &http.Client{}

	response, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Could not execute request: %s", err)
	}

	defer response.Body.Close()

	return response, nil
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
