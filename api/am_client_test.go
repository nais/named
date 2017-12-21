package api

import (
	"github.com/golang/glog"
	"github.com/h2non/gock"
	"github.com/stretchr/testify/assert"
	"testing"
)

var baseUrl = "https://server.domain.com"
var amc = AMConnection{BaseURL: baseUrl, User: "user", Password: "pass"}

func TestRest(t *testing.T) {

	defer gock.Off()

	gock.New(baseUrl).
		Post("/json/authenticate").
		MatchHeader("Content-Type", "application/json").
		MatchHeader("X-OpenAM-Username", "user").
		MatchHeader("X-OpenAM-Password", "pass").
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
		Post("/json/authenticate").
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
		glog.Errorf("Could not authenticate on AM server %s: %s", baseUrl, err)
	}

	json, err := amc.ExportPolicies("json", "%2F")
	if err != nil {
		glog.Errorf("Could not list resource types %s", err)
	}
	glog.Infof(json)
	assert.NotEmpty(t, json)
	assert.True(t, true, gock.IsDone())
}
