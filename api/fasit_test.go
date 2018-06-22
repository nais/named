package api

import (
	"testing"

	"encoding/json"
	"github.com/h2non/gock"
	"github.com/stretchr/testify/assert"
)

func TestGettingResource(t *testing.T) {
	alias := "openam"
	resourceType := "OpenAm"
	environment := "testname"
	application := "openam"
	zone := "zone"
	hostname := "hostname.domain.com"
	username := "user"

	fasit := FasitClient{"https://fasit.local", "", ""}

	defer gock.Off()

	gock.New("https://fasit.local").
		Get("/api/v2/scopedresource").
		MatchParam("alias", alias).
		MatchParam("type", resourceType).
		MatchParam("environment", environment).
		MatchParam("application", application).
		MatchParam("zone", zone).
		Reply(200).File("testdata/fasitAmResponse.json")

	resource, err := fasit.GetOpenAmResource(ResourceRequest{alias, resourceType}, environment, application, zone)

	assert.Nil(t, err)
	assert.Equal(t, hostname, resource.Hostname)
	assert.Equal(t, username, resource.Username)
}

func TestGetFasitApplication(t *testing.T) {
	fasit := FasitClient{"https://fasit.local", "", ""}

	defer gock.Off()

	gock.New("https://fasit.local").
		Get("/api/v2/applications/testapp").
		Reply(200)

	gock.New("https://fasit.local").
		Get("/api/v2/applications/appdoesontexist").
		Reply(404)

	assert.Nil(t, fasit.GetFasitApplication("testapp"))
	assert.Error(t, fasit.GetFasitApplication("appdoesnotexist"))
}

func TestGetFasitEnvironment(t *testing.T) {
	fasit := FasitClient{"https://fasit.local", "", ""}

	defer gock.Off()

	gock.New("https://fasit.local").
		Get("/api/v2/environments/testenv").
		Reply(200).BodyString("{\"environmentclass\": \"u\"}")

	gock.New("https://fasit.local").
		Get("/api/v2/environments/envdoesontexist").
		Reply(404)

	envClass1, err1 := fasit.GetFasitEnvironment("testenv")
	assert.Equal(t, "u", envClass1)
	assert.Nil(t, err1)

	envClass2, err2 := fasit.GetFasitEnvironment("envdoesontexist")
	assert.Empty(t, envClass2)
	assert.Error(t, err2)
	assert.Equal(t, "Item not found in Fasit: https://fasit.local/api/v2/environments/envdoesontexist", err2.Message)
}

func TestGetIngressUrl(t *testing.T) {
	application := "testapp"
	environmentName := "testname"
	zone := "fss"
	fasit := FasitClient{"https://fasit.local", "", ""}

	defer gock.Off()

	gock.New("https://fasit.local").
		Get("/api/v2/environments/testname").
		Reply(200).BodyString("{\"environmentclass\": \"q\"}")

	request := NamedConfigurationRequest{Application: application, Environment: environmentName}
	urls, err := fasit.GetIngressURL(&request, zone)
	assert.Equal(t, []string{"testapp.nais.preprod.local", "testapp-" + environmentName + ".nais.preprod.local"}, urls)
	assert.Nil(t, err)
}

func TestGetDomainFromZoneAndEnvironmentClass(t *testing.T) {
	assert.Equal(t, "preprod.local", GetDomainFromZoneAndEnvironmentClass("t", "fss"))
	assert.Equal(t, "preprod.local", GetDomainFromZoneAndEnvironmentClass("q", "null"))
	assert.Equal(t, "oera-q.local", GetDomainFromZoneAndEnvironmentClass("null", "sbs"))
	assert.Equal(t, "oera-q.local", GetDomainFromZoneAndEnvironmentClass("t", "sbs"))
	assert.Equal(t, "adeo.no", GetDomainFromZoneAndEnvironmentClass("p", "fss"))
	assert.Equal(t, "oera.no", GetDomainFromZoneAndEnvironmentClass("p", "sbs"))
}

func TestPostFasitResources(t *testing.T) {
	fasit := FasitClient{"https://fasit.local", "", ""}
	issoResource := IssoResource{
		oidcURL:           "oidcURL",
		IssoIssuerURL:     "issoIssuerURL",
		IssoJwksURL:       "issoJwksURL",
		oidcUsername:      "oidcUsername",
		oidcAgentPassword: "oicdAgentPassword",
	}
	namedRequest := NamedConfigurationRequest{
		Application: "appName",
		Version:     "123",
		Environment: "cd-u1",
		Username:    "test",
		Password:    "password",
	}

	defer gock.Off()

	gock.New("https://fasit.local").
		Get("/api/v2/environments/cd-u1").
		Reply(200).BodyString("{\"environmentclass\": \"t\"}")

	payload, fasitErr := fasit.CreateFasitResourceForOpenIDConnect(issoResource, &namedRequest, "fss")
	assert.Nil(t, fasitErr)

	t.Run("Test if payload is created correctly", func(t *testing.T) {
		asJSON, err := json.Marshal(payload)
		assert.NoError(t, err)
		assert.Equal(t, "{\"ID\":0,\"alias\":\"appName-oidc\",\"type\":\"OpenIdConnect\"," +
			"\"scope\":{\"environmentclass\":\"t\",\"environment\":\"cd-u1\",\"zone\":\"fss\"," +
			"\"application\":\"appName\"},\"properties\":{\"agentName\":\"appName-cd-u1\",\"hostUrl\":\"oidcURL\"," +
			"\"issuerUrl\":\"issoIssuerURL\",\"jwksUrl\":\"issoJwksURL\"},\"secrets\":{\"password\":{\"value\":\"oicdAgentPassword\"}}}", string(asJSON))
	})

	t.Run("POSTing openIDConnect resource", func(t *testing.T) {
		gock.New("https://fasit.local").
			Post("/api/v2/resources").
			Reply(201)

		appErr := fasit.PostFasitResource(payload, &namedRequest)
		assert.Nil(t, appErr)
	})
}
