package api

import (
	"encoding/json"
	"fmt"
	"github.com/golang/glog"
	"io/ioutil"
	"net/http"
	"bytes"
	"io"
	"strings"
)

type OpenAMConnection struct {
	BaseURL		string
	User 		string
	Password 	string
	tokenId		string
	Realm		string
}

type AuthNResponse struct {
	TokenID		string `json:"tokenId"`
	SuccessURL	string `json:"successUrl"`
}

func GetOpenAMConnection() (am *OpenAMConnection, err error) {
	url := GetOpenAMUrl()
	user := GetOpenAMUser()
	pass := GetOpenAMPassword()
	return Open(url, user, pass)
}

func Open(url, user, password string) (am *OpenAMConnection, err error) {
	am = &OpenAMConnection{BaseURL:url, User:user, Password:password}
	err = am.Authenticate()

	return
}

func (am *OpenAMConnection) Authenticate() error {
	url := am.requestURL("/json/authenticate")

	var jsonStr = []byte(`{}`)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonStr))

	if err != nil {
		fmt.Errorf("Something happened: ", err)
	}
	req.Header.Set("X-OpenAM-Username", am.User)
	req.Header.Set("X-OpenAM-Password", am.Password)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}

	fmt.Printf("Authenticating to OpenAM ", url)
	response, err := client.Do(req)
	if err != nil {
		return err
	}

	defer response.Body.Close()

	body, _ := ioutil.ReadAll(response.Body)
	var a AuthNResponse

	err = json.Unmarshal(body, &a)
	if response.StatusCode != 200 {
		return fmt.Errorf("Failed to authenticate. %v", response.Status)
	}

	am.tokenId = a.TokenID

	return nil
}

func (am *OpenAMConnection) requestURL( path string) string  {
	var strs []string
	strs = append(strs, am.BaseURL)
	strs = append(strs, path)
	return strings.Join(strs, "")
}

func (openam *OpenAMConnection) newRequest(method, url string, body io.Reader) *http.Request {
	request, err := http.NewRequest(method, openam.requestURL(url), body)
	if err != nil {glog.Errorf("Could not create new request, error: %v", err)}

	iPlanetCookie := http.Cookie{Name: "iPlanetDirectoryPro", Value: openam.tokenId}
	request.AddCookie(&iPlanetCookie)
	request.Header.Set("Content-Type", "application/json")
	return request
}

