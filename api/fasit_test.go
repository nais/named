package api

import (
	"fmt"
	"testing"

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

type FakeFasitClient struct {
	FasitUrl string
	FasitClient
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
	urls, err := fasit.GetIngressUrl(&request, zone)
	assert.Equal(t, []string{"testapp.nais.preprod.local", "testapp-" + environmentName + ".nais.preprod.local"}, urls)
	assert.Nil(t, err)
}

func TestGetDomainFromZoneAndEnvironmentClass(t *testing.T) {
	assert.Equal(t, "devillo.no", GetDomainFromZoneAndEnvironmentClass("q", "null"))
	assert.Equal(t, "preprod.local", GetDomainFromZoneAndEnvironmentClass("t", "fss"))
}

func (fasit FakeFasitClient) getScopedResource(resourcesRequest ResourceRequest, environment, application, zone string) (OpenAmResource, AppError) {
	switch application {
	case "notfound":
		return OpenAmResource{}, appError{fmt.Errorf("not found"), "Resource not found in Fasit", 404}
	case "fasitError":
		return OpenAmResource{}, appError{fmt.Errorf("error from fasit"), "random error", 500}
	default:
		return OpenAmResource{}, nil
	}
}
