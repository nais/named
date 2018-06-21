package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAnIncorrectPayloadGivesError(t *testing.T) {
	api := API{}
	body := strings.NewReader("gibberish")

	req, err := http.NewRequest("POST", "/configure", body)
	if err != nil {
		panic("could not create req")
	}

	rr := httptest.NewRecorder()
	handler := http.Handler(appHandler(api.configure))
	handler.ServeHTTP(rr, req)

	assert.Equal(t, 400, rr.Code)
}

func TestInvalidFasit(t *testing.T) {
	api := API{"https://fasit.local", "testCluster"}
	jsn, _ := json.Marshal(CreateConfigurationRequest("appname", "123", "env", "test", "test", []string{"/test"}))

	body := strings.NewReader(string(jsn))
	req, err := http.NewRequest("POST", "/configure", body)
	if err != nil {
		panic("could not create req")
	}

	rr := httptest.NewRecorder()
	handler := http.Handler(appHandler(api.configure))
	handler.ServeHTTP(rr, req)
	assert.Contains(t, rr.Body.String(), "Error contacting fasit")
}

func TestCheckIfInvalidZone(t *testing.T) {
	zone1 := GetZone("cluster")
	assert.Empty(t, zone1)

	zone2 := GetZone("preprod-fss")
	assert.Equal(t, "fss", zone2)
}

func TestValidateDeploymentRequest(t *testing.T) {
	t.Run("Empty fields should be marked invalid", func(t *testing.T) {
		invalid := CreateConfigurationRequest("", "", "", "", "", []string{})

		err := invalid.Validate("fss")

		assert.NotNil(t, err)
		assert.Contains(t, err, errors.New("application is required but empty"))
		assert.Contains(t, err, errors.New("version is required but empty"))
		assert.Contains(t, err, errors.New("environment is required but empty"))
		assert.Contains(t, err, errors.New("username is required but empty"))
		assert.Contains(t, err, errors.New("password is required but empty"))
		assert.Contains(t, err, errors.New("contextRoots are required but empty"))
	})
}

func CreateConfigurationRequest(appName, version, env, username, password string, urls []string) NamedConfigurationRequest {
	return NamedConfigurationRequest{
		Application:  appName,
		Version:      version,
		Environment:  env,
		Username:     username,
		Password:     password,
		ContextRoots: urls,
	}
}

func TestFasitResourceForOpenIDConnect(t *testing.T) {
	issoResource := IssoResource{
		oidcURL:           "oidcURL",
		IssoIssuerURL:     "issoIssuerURL",
		IssoJwksURL:       "issoJwksURL",
		oidcUsername:      "oidcUsername",
		oidcAgentPassword: "oicdAgentPassword",
	}
	request := CreateConfigurationRequest("appname", "123", "cd-u1", "test", "password", []string{"/test"})
	payload := createFasitResourceForOpenIDConnect("t", issoResource, &request, "fss")

	t.Run("Test if payload is created correctly", func(t *testing.T) {
		asJSON, err := json.Marshal(payload)
		assert.NoError(t, err)
		assert.Equal(t, string(asJSON), "{\"ID\":0,\"alias\":\"appname-oidc\",\"type\":\"OpenIdConnect\",\"scope\":{\"environmentclass\":\"t\",\"environment\":\"cd-u1\",\"zone\":\"fss\",\"application\":\"appname\"},\"properties\":{\"agentName\":\"oidcUsername\",\"hostUrl\":\"oidcURL\",\"issuerUrl\":\"issoIssuerURL\",\"jwksUrl\":\"issoJwksURL\"},\"secrets\":{\"password\":{\"value\":\"oicdAgentPassword\"}}}")
	})
}
