package api

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/golang/glog"
	"github.com/h2non/gock"
	"github.com/stretchr/testify/assert"
)

var baseUrl = "https://server.domain.com"
var authUrl = "/json/authenticate"
var policyUrl = "/json/policies"
var amc = AMConnection{BaseURL: baseUrl, User: "user", Password: "pass"}

func TestGetRequestUrlShouldReturnConcatenatedString(t *testing.T) {
	assert.Equal(t, baseUrl+authUrl, amc.getRequestURL(authUrl))
}

func TestCreateNewRequestShouldReturnRequestWIthCookie(t *testing.T) {
	testRequest, err := amc.createNewRequest("GET", baseUrl+policyUrl, nil)
	assert.Nil(t, err)
	assert.True(t, testRequest.Header.Get("Content-type") == "application/json")
	assert.True(t, len(testRequest.Cookies()) == 1)
}

func TestAgentExists(t *testing.T) {

	defer gock.Off()

	gock.New(baseUrl).
		Get("/json/agents/testAgent").
		MatchHeader("nav-isso", amc.tokenId).
		Reply(200)

	gock.New(baseUrl).
		Get("/json/agents/noTestAgent").
		MatchHeader("nav-isso", amc.tokenId).
		Reply(404)

	assert.True(t, amc.AgentExists("testAgent"))
	assert.False(t, amc.AgentExists("noTestAgent"))
}

func TestCreateAgent(t *testing.T) {

	payload, _ := json.Marshal(buildAgentPayload("testAgent", "", []string{}))

	defer gock.Off()

	gock.New(baseUrl).
		Post("/json/agents/").
		MatchHeader("nav-isso", amc.tokenId).
		MatchParam("_action", "create").
		Body(bytes.NewReader(payload)).
		Reply(200)

	err := amc.CreateAgent("testAgent", []string{}, &IssoResource{}, &NamedConfigurationRequest{})
	assert.NoError(t, err)
}

func TestDeleteAgent(t *testing.T) {

	defer gock.Off()

	gock.New(baseUrl).
		Delete("/json/agents/testAgent").
		MatchHeader("nav-isso", amc.tokenId).
		Reply(200)

	gock.New(baseUrl).
		Delete("/json/agents/noTestAgent").
		MatchHeader("nav-isso", amc.tokenId).
		Reply(404)

	assert.Nil(t, amc.DeleteAgent("testAgent"))
	assert.NotNil(t, amc.DeleteAgent("noTestAgent"))
}

func TestFormatAmHeaderString(t *testing.T) {
	headerString := "user"
	formattedHeaderString := FormatAmHeaderString(headerString)
	expectedResult := "=?UTF-8?B?" + base64.StdEncoding.EncodeToString([]byte(headerString)) + "?="
	assert.Equal(t, expectedResult, formattedHeaderString)
}

func TestCreateRedirectionUris(t *testing.T) {
	request := NamedConfigurationRequest{ContextRoots: []string{"/testapp", "testapp2"}}
	issoResource := IssoResource{loadbalancerUrl: "nais.example.com", ingressUrls: []string{"test.test.domain", "testapp.test.domain"}}
	uriList := CreateRedirectionUris(&issoResource, &request)
	assert.Len(t, uriList, 6)
	assert.Contains(t, uriList, "[0]=https://test.test.domain/testapp")
	assert.Contains(t, uriList, "[1]=https://testapp.test.domain/testapp")
	assert.Contains(t, uriList, "[2]=https://nais.example.com/testapp")
	assert.Contains(t, uriList, "[3]=https://test.test.domain/testapp2")
	assert.Contains(t, uriList, "[4]=https://testapp.test.domain/testapp2")
	assert.Contains(t, uriList, "[5]=https://nais.example.com/testapp2")
}

func TestRest(t *testing.T) {

	defer gock.Off()

	gock.New(baseUrl).
		Post(authUrl).
		MatchHeader("Content-Type", "application/json").
		MatchHeader("X-OpenAM-Username", "user").
		MatchHeader("X-OpenAM-Password", "pass").
		MatchHeader("Cache-Control", "no-cache").
		MatchParam("authIndexType", "service").
		MatchParam("authIndexValue", "adminconsoleservice").
		Reply(200)

	gock.New(baseUrl).
		Get("/json/resourcetypes").
		MatchParam("_queryFilter", "true").
		Reply(200).
		File("testdata/amResourceTypes.json")

	err := amc.Authenticate()
	if err != nil {
		glog.Errorf("Could not authenticate on AM server %s: %s", baseUrl, err)
	}

	rt, err := amc.ListResourceTypes()

	if err != nil {
		glog.Errorf("Could not list resource types %s", err)
	}

	for _, v := range rt {
		glog.Infof("%v", v)
	}
	assert.NotEmpty(t, rt)
	assert.True(t, true, gock.IsDone())
}

func TestJsonExport(t *testing.T) {

	defer gock.Off()

	gock.New(baseUrl).
		Post(authUrl).
		MatchHeader("Content-Type", "application/json").
		MatchHeader("X-OpenAM-Username", "user").
		MatchHeader("X-OpenAM-Password", "pass").
		Reply(200)

	gock.New(baseUrl).
		Get("/json/policies").
		MatchParam("_queryFilter", "true").
		Reply(200).
		File("testdata/amPolicyExport.json")

	if err := amc.Authenticate(); err != nil {
		glog.Errorf("could not authenticate on AM server %s: %s", baseUrl, err)
	}

	jsn, err := amc.ExportPolicies("json", "%2F")
	if err != nil {
		glog.Errorf("could not list resource types %s", err)
	}
	glog.Infof(jsn)
	assert.NotEmpty(t, jsn)
	assert.True(t, true, gock.IsDone())
}
