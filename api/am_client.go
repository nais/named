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

type AgentPayload struct {
	Username string `json:"username"`
	Password string `json:"userpassword"`
	AgentType string `json:"agenttype"`
	Algorithm string `json:"com.forgerock.openam.oauth2provider.idTokenSignedResponseAlg"`
	RedirectionUris []string `json:"com.forgerock.openam.oauth2provider.redirectionURIs"`
	Scope string `json:"com.forgerock.openam.oauth2provider.scopes"`
	ConsentImplied string `json:"isConsentImplied"`
}

// GetAmConnection returns connection to AM server
func GetAmConnection(issoResource IssoResource) (am *AMConnection, err error) {
	return open(issoResource.oidcUrl, issoResource.oidcUsername, issoResource.oidcPassword)
}

func open(url, user, password string) (am *AMConnection, err error) {
	am = &AMConnection{BaseURL: url, User: user, Password: password}
	err = am.Authenticate()
	return am, err
}

// Authenticate connects to AM server and sets tokenID in AMConnection struct
func (am *AMConnection) Authenticate() (error) {
	url := am.requestURL("/json/authenticate?authIndexType=service&authIndexValue=adminconsoleservice")

	var jsonStr = []byte(`{}`)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonStr))
	if err != nil {
		glog.Errorf("Could not create request: %s", err)
	}

	req.Header.Set("X-OpenAM-Username", am.User)
	req.Header.Set("X-OpenAM-Password", am.Password)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}

	glog.Infof("Authenticating to AM %s", url)
	response, err := client.Do(req)
	if err != nil {
		return err
	}

	defer response.Body.Close()

	body, _ := ioutil.ReadAll(response.Body)
	var a AuthNResponse

	err = json.Unmarshal(body, &a)
	if response.StatusCode != 200 {
		return fmt.Errorf("Failed to authenticate %v: %s", response.Status, err)
	}

	am.tokenId = a.TokenID

	return nil
}

func (am *AMConnection) requestURL(path string) string {
	var strs []string
	strs = append(strs, am.BaseURL)
	strs = append(strs, path)
	return strings.Join(strs, "")
}

func (am *AMConnection) newRequest(method, url string, body io.Reader) (*http.Request, error) {
	request, err := http.NewRequest(method, am.requestURL(url), body)
	if err != nil {
		return request, fmt.Errorf("Could not create new request, error: %v", err)
	}

	iPlanetCookie := http.Cookie{Name: "iPlanetDirectoryPro", Value: am.tokenId}
	request.AddCookie(&iPlanetCookie)
	request.Header.Set("Content-Type", "application/json")
	return request, nil
}

func (am *AMConnection) agentExists(agentName string) bool {
	agentUrl := am.BaseURL + "/json/agents/" + agentName
	req, err := http.NewRequest(http.MethodGet, agentUrl, nil)
	if err != nil {
		glog.Errorf("Could not create request: %s", err)
	}
	req.Header.Set("nav-isso", am.tokenId)
	client := &http.Client{}

	glog.Infof("Checking if agent exists %s", agentName)
	response, err := client.Do(req)
	if err != nil {
		glog.Errorf("Could not execute request: %s", err)
		return false
	}

	defer response.Body.Close()

	body, _ := ioutil.ReadAll(response.Body)
	var a AuthNResponse

	err = json.Unmarshal(body, &a)
	if response.StatusCode == 200 {
		return true
	}
	return false
}

func (am *AMConnection) createAgent(redirectionUris []string) (bool, error) {
	agentUrl := am.BaseURL + "/json/agents/"
	payload, err := json.Marshal(buildAgentPayload(am, redirectionUris))

	req, err := http.NewRequest(http.MethodPost, agentUrl, bytes.NewBuffer(payload))
	if err != nil {
	glog.Errorf("Could not create request: %s", err)
	}
	req.Header.Set("nav-isso", am.tokenId)
	client := &http.Client{}

	response, err := client.Do(req)
	if err != nil {
	return false, fmt.Errorf("Could not execute request to create agent: %s", err)
	}

	defer response.Body.Close()

	body, _ := ioutil.ReadAll(response.Body)
	var a AuthNResponse

	err = json.Unmarshal(body, &a)
	if response.StatusCode != 200 {
		return false, fmt.Errorf("Agent %s could not be created: %s", err)
	}
	return true, nil

}

func buildAgentPayload(am *AMConnection, uris []string) AgentPayload {
	agentPayload := AgentPayload{
		Username: am.User,
		Password: am.Password,
		AgentType: "OAuth2Client",
		Algorithm: "RS256",
		Scope: "[0]=openid",
		ConsentImplied: "true",
		RedirectionUris: []string{},
	}

	return agentPayload
}

func (am *AMConnection) deleteAgent(agentName string) error {
	agentUrl := am.BaseURL + "/json/agents/" + agentName
	req, err := http.NewRequest(http.MethodDelete, agentUrl, nil)
	if err != nil {
		glog.Errorf("Could not create request: %s", err)
	}
	req.Header.Set("nav-isso", am.tokenId)
	client := &http.Client{}

	glog.Infof("Deleting agent %s", agentName)
	response, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("Could not execute request to delete agent: %s", err)
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