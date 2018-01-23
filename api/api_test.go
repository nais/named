package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	/*	"bytes"
		"fmt"
		"golang.org/x/crypto/ssh"
		"github.com/h2non/gock"*/)

func TestAnIncorrectPayloadGivesError(t *testing.T) {
	api := Api{}
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

func TestInvalidZone(t *testing.T) {
	api := Api{"https://fasit.local", "testCluster"}
	json, _ := json.Marshal(CreateConfigurationRequest("appname", "123", "env", "zone1", "test", "test"))

	body := strings.NewReader(string(json))
	req, err := http.NewRequest("POST", "/configure", body)
	if err != nil {
		panic("could not create req")
	}

	rr := httptest.NewRecorder()
	handler := http.Handler(appHandler(api.configure))
	handler.ServeHTTP(rr, req)

	assert.Equal(t, "Zone has to be fss or sbs, not zone1: No AM configurations available for this zone (400)\n", rr.Body.String())

}

func TestCheckIfInvalidZone(t *testing.T) {
	api := Api{"https://fasit.local", "testCluster"}
	request := NamedConfigurationRequest{Zone: "fss"}
	result, err := api.checkIfInvalidRequest(request)
	assert.NotNil(t, err)
	assert.True(t, result)

	api = Api{"https://fasit.local", "preprod-fss"}
	result, err = api.checkIfInvalidRequest(request)
	assert.Nil(t, err)
	assert.False(t, result)
}

/*
func TestSsh(t *testing.T) {
	sshClient, sshSession, err := SshConnect("user", "pass", "hostname.com", "22")
	if err != nil {
		fmt.Printf("Could not get ssh session ", err)
	}
	defer sshSession.Close()
	defer sshClient.Close()

	modes := ssh.TerminalModes{
		ssh.ECHO:  0, // Disable echoing
		ssh.IGNCR: 1, // Ignore CR on input.
	}

	if err := sshSession.RequestPty("xterm", 80, 40, modes); err != nil {
		fmt.Printf("Could not set pty")
	}
	cmd := fmt.Sprintf("sudo python /opt/openam/scripts/openam_policy.py %s %s", "nais-testapp", "nais-testapp")
	var stdoutBuf bytes.Buffer
	var stderrBuf bytes.Buffer

	sshSession.Stdout = &stdoutBuf
	sshSession.Stderr = &stderrBuf

	fmt.Printf("Running command %s", cmd)
	error := sshSession.Run(cmd)
	fmt.Printf(stderrBuf.String() + "test" +  stdoutBuf.String())
	if error != nil {
		fmt.Errorf("Could not run command %s %s", cmd, error)
	}
}


func TestValidConfigurationRequestInSBS(t *testing.T) {
	appName := "appname"
	environment := "environmentName"
	version := "123"
	resourceAlias := "OpenAM"
	resourceType := "OpenAM"
	zone := "sbs"
	username := "user"
	password := "pass"
	api := Api{"https://fasit.local", "testCluster"}

	defer gock.Off()

	gock.New("https://fasit.local").
		Get("/api/v2/scopedresource").
		MatchParam("alias", resourceAlias).
		MatchParam("type", resourceType).
		MatchParam("environment", environment).
		MatchParam("application", appName).
		MatchParam("zone", zone).
		Reply(200).File("testdata/fasitAmResponse.json")

	gock.New("https://repo.adeo.no").
		Get("/repository/raw/nais/appname/123/am/not-enforced-urls.txt").
		Reply(200).File("testdata/not-enforced-urls.txt")

	gock.New("https://repo.adeo.no").
		Get("/repository/raw/nais/appname/123/am/app-policies.xml").
		Reply(200).File("testdata/app-policies.xml")

	assert.True(t, gock.IsPending())
	json, _ := json.Marshal(CreateConfigurationRequest(appName, version, environment, zone, username, password))

	body := strings.NewReader(string(json))
	req, _ := http.NewRequest("POST", "/configure", body)

	rr := httptest.NewRecorder()
	handler := http.Handler(appHandler(api.configure))
	handler.ServeHTTP(rr, req)

	assert.Equal(t, 200, rr.Code)
	assert.True(t, gock.IsDone())
	assert.Equal(t, "", string(rr.Body.Bytes()))
}
*/

func TestValidateDeploymentRequest(t *testing.T) {
	t.Run("Empty fields should be marked invalid", func(t *testing.T) {
		invalid := CreateConfigurationRequest("", "", "", "", "", "")

		err := invalid.Validate()

		assert.NotNil(t, err)
		assert.Contains(t, err, errors.New("Application is required and is empty"))
		assert.Contains(t, err, errors.New("Version is required and is empty"))
		assert.Contains(t, err, errors.New("Environment is required and is empty"))
		assert.Contains(t, err, errors.New("Zone is required and is empty"))
		assert.Contains(t, err, errors.New("Zone can only be fss, sbs or iapp"))
		assert.Contains(t, err, errors.New("Username is required and is empty"))
		assert.Contains(t, err, errors.New("Password is required and is empty"))
	})
}

func CreateConfigurationRequest(appName, version, env, zone, username, password string) NamedConfigurationRequest {
	return NamedConfigurationRequest{
		Application: appName,
		Version:     version,
		Environment: env,
		Zone:        zone,
		Username:    username,
		Password:    password,
	}
}
