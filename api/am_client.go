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

// GetAmConnection returns connection to AM server
func GetAmConnection() (am *AMConnection, err error) {
	url := GetAmUrl()
	user := GetAmUser()
	pass := GetAmPassword()
	return open(url, user, pass)
}

func open(url, user, password string) (am *AMConnection, err error) {
	am = &AMConnection{BaseURL: url, User: user, Password: password}
	err = am.Authenticate()

	return
}

// Authenticate connects to AM server and sets tokenID in AMConnection struct
func (am *AMConnection) Authenticate() error {
	url := am.requestURL("/json/authenticate")

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
