package api

import (
	"fmt"
	"github.com/h2non/gock"
	"github.com/stretchr/testify/assert"
	"testing"
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

func TestGetFasitEnvironment(t *testing.T) {
	namespace := "namespace"

	defer gock.Off()
	gock.New("https://fasit.local").
		Get("/api/v2/environments/" + namespace).
		Reply(200).
		JSON(map[string]string{"environmentclass": "u"})

	fasit := FasitClient{"https://fasit.local", "", ""}
	t.Run("Returns an error if environment isn't found", func(t *testing.T) {
		_, err := fasit.GetFasitEnvironment("notExisting")
		assert.Error(t, err)
		assert.False(t, gock.IsDone())
	})
	t.Run("Returns no error if environment is found", func(t *testing.T) {
		_, err := fasit.GetFasitEnvironment(namespace)
		assert.NoError(t, err)
		assert.True(t, gock.IsDone())
	})

}
