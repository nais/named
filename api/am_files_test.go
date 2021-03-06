package api

import (
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"github.com/h2non/gock"
	"github.com/stretchr/testify/assert"
)

func TestGetAmFiles(t *testing.T) {
	const policypath = "https://repo.adeo.no/repository/raw/nais/testapp/2" +
		".0/am/app-policies.xml"
	const notenforcedpath = "https://repo.adeo.no/repository/raw/nais/testapp/2" +
		".0/am/not-enforced-urls.txt"
	defer gock.Off()

	gock.New(policypath).Reply(200).File("testdata/app-policies.xml")
	gock.New(notenforcedpath).Reply(200).File("testdata/not-enforced-urls.txt")
	files, err := GenerateAmFiles(&NamedConfigurationRequest{Application: "testapp", Version: "2.0"})

	assert.NoError(t, err)
	assert.Equal(t, 2, len(files))
	assert.Equal(t, "/tmp/testapp/app-policies.xml", files[0])
	assert.Equal(t, "/tmp/testapp/not-enforced-urls.txt", files[1])
}

func TestUpdatePolicyFiles(t *testing.T) {
	policyFiles := []string{"testdata/app-policies.xml", "testdata/not-enforced-urls.txt"}
	environment := "u653"

	err := UpdatePolicyFiles(policyFiles, environment)
	assert.Nil(t, err)
	file1, _ := ioutil.ReadFile(policyFiles[0])
	file2, _ := ioutil.ReadFile(policyFiles[1])
	assert.False(t, strings.Contains(string(file1), "DomainName"))
	assert.False(t, strings.Contains(string(file2), "DomainName"))
	assert.True(t, strings.Contains(string(file1), "tjenester-u653.nav.no"))
}

func TestValidFileGivesNoError(t *testing.T) {
	xmlFileName := "testdata/app-policies.xml"
	txtFileName := "testdata/not-enforced-urls.txt"

	assert.Nil(t, validateContent(xmlFileName))
	assert.Nil(t, validateContent(txtFileName))
}

func TestInvalidFileGivesError(t *testing.T) {
	fileName := "testdata/app-policies-error.xml"

	err := validateContent(fileName)
	assert.Equal(t, "Unknown file type", err.ErrorMessage)
}

func TestCleanupNonExistingFilesGivesError(t *testing.T) {
	policyFiles := []string{"testdata/app-policy-does-not-exist.xml"}
	err := cleanupLocalFiles(policyFiles)
	assert.Equal(t, "could not remove file: testdata/app-policy-does-not-exist.xml", err.Error())
}

func TestFetchNonExistingFilesShouldReturnError(t *testing.T) {
	app := "testapp"
	version := "2.0"
	urls := createPolicyFileUrls(app, version)

	assert.Equal(t, "https://repo.adeo.no/repository/raw/nais/testapp/2"+
		".0/am/app-policies.xml", urls[0])
	assert.Equal(t, "https://repo.adeo.no/repository/raw/nais/testapp/2"+
		".0/am/not-enforced-urls.txt", urls[1])

	_, err := fetchPolicyFiles(urls, app)
	assert.NotNil(t, err)

	_, fileErr := os.Stat("/tmp/" + app)
	assert.Nil(t, fileErr)
}
