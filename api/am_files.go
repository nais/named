package api

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/golang/glog"
	"github.com/h2non/filetype"
	"golang.org/x/crypto/ssh"
)

// ValidationErrors contains all validation errors
type ValidationErrors struct {
	Errors []ValidationError
}

// ValidationError contains error and fields of destruction
type ValidationError struct {
	ErrorMessage string
	Fields       map[string]string
}

// GenerateAmFiles returns array of validated and downloaded policy files
func GenerateAmFiles(request *NamedConfigurationRequest) ([]string, error) {
	policyFiles, err := downloadPolicies(request)
	if err != nil {
		return []string{}, err
	}

	validationErrors := ValidatePolicyFiles(policyFiles)
	if len(validationErrors.Errors) != 0 {
		return []string{}, validationErrors
	}

	return policyFiles, nil
}

func downloadPolicies(request *NamedConfigurationRequest) ([]string, error) {
	urls := createPolicyFileUrls(request.Application, request.Version)
	files, err := fetchPolicyFiles(urls, request.Application)
	if err != nil {
		return []string{}, err
	}

	return files, nil
}

func createPolicyFileUrls(application, version string) []string {
	var urls = []string{}
	baseUrl := "https://repo.adeo.no/repository/raw/nais"
	urls = append(urls, fmt.Sprintf("%s/%s/%s/am/app-policies.xml", baseUrl, application, version))
	urls = append(urls, fmt.Sprintf("%s/%s/%s/am/not-enforced-urls.txt", baseUrl, application, version))
	return urls
}

func fetchPolicyFiles(urls []string, application string) ([]string, error) {
	var fileNames = []string{}
	for _, url := range urls {
		glog.Infof("Fetching file from URL %s\n", url)

		_, fileName := filepath.Split(url)

		if _, err := os.Stat("/tmp/" + application); os.IsNotExist(err) {
			os.Mkdir("/tmp/"+application, os.ModePerm)
		}

		out, err := os.Create("/tmp/" + application + "/" + fileName)
		if err != nil {
			return []string{}, fmt.Errorf("Could not create file %s " + fileName)
		}

		defer out.Close()

		response, err := http.Get(url)
		if err != nil {
			return []string{}, fmt.Errorf("HTTP GET failed for url: %s. %s", url, err)
		}

		defer response.Body.Close()

		if response.StatusCode > 299 {
			return []string{}, fmt.Errorf("got HTTP status code %d fetching manifest from URL: %s", response.StatusCode,
				url)
		}

		_, err = io.Copy(out, response.Body)
		if err != nil {
			return []string{}, fmt.Errorf("Could not write to %s ", fileName)
		}

		fileNames = append(fileNames, out.Name())
	}

	return fileNames, nil
}

// CopyFilesToAmServer sftps policy files to desired AM host
func CopyFilesToAmServer(sshClient *ssh.Client, policyFiles []string, application string) error {
	sftpClient, err := SftpConnect(sshClient)
	if err != nil {
		return fmt.Errorf("could not transfer files to AM server: %s", err)
	}

	for _, policyFile := range policyFiles {
		srcFile, err := os.Open(policyFile)
		if err != nil {
			return fmt.Errorf("could not openAdminConnection file %s: %s", policyFile, err)
		}

		srcFileInfo, err := srcFile.Stat()
		if err != nil {
			return fmt.Errorf("could not stat file %s: %s", policyFile, err)
		}

		defer srcFile.Close()

		_ = sftpClient.Mkdir("/tmp/" + application)
		destFile, err := sftpClient.Create(policyFile)
		if err != nil {
			return fmt.Errorf("could not create am file %s: %s", policyFile, err)
		}
		defer destFile.Close()

		buf := make([]byte, srcFileInfo.Size())
		for {
			n, _ := srcFile.Read(buf)
			if n == 0 {
				break
			}
		}
		destFile.Write(buf)
	}

	cleanupLocalFiles(policyFiles)
	return nil
}

// UpdatePolicyFiles replaces ${DomainName} with correct site name in policy files
func UpdatePolicyFiles(policyFiles []string, environment string) error {
	siteName := "tjenester.nav.no"
	if strings.ToLower(environment[:1]) != "p" {
		siteName = "tjenester-" + environment + ".nav.no"
	}

	for _, policyFile := range policyFiles {
		read, err := ioutil.ReadFile(policyFile)
		if err != nil {
			return fmt.Errorf("could not read file %s", policyFile)
		}

		newContents := strings.Replace(string(read), "${DomainName}", siteName, -1)

		err = ioutil.WriteFile(policyFile, []byte(newContents), 0)
		if err != nil {
			return fmt.Errorf("could not write file %s", policyFile)
		}

	}

	return nil
}

func cleanupLocalFiles(policyFiles []string) error {
	for _, fileName := range policyFiles {
		err := os.Remove(fileName)
		if err != nil {
			return fmt.Errorf("could not remove file: %s", fileName)
		}
	}
	return nil
}

// ValidatePolicyFiles validates the policy xml files, checking the file type
func ValidatePolicyFiles(fileNames []string) ValidationErrors {
	var validationErrors ValidationErrors

	for _, fileName := range fileNames {
		validations := []func(string) *ValidationError{
			validateContent,
		}

		for _, valfunc := range validations {
			if valError := valfunc(fileName); valError != nil {
				validationErrors.Errors = append(validationErrors.Errors, *valError)
			}
		}

	}
	return validationErrors
}

func validateContent(fileName string) *ValidationError {
	addMatchers()
	buf, _ := ioutil.ReadFile(fileName)
	kind, _ := filetype.Match(buf)

	if fileName[len(fileName)-3:] == "txt" {
		return nil
	}

	if kind.Extension == "unknown" {
		return &ValidationError{
			"Unknown file type",
			map[string]string{"File": fileName},
		}
	}

	return nil
}

func (errors ValidationErrors) Error() (s string) {
	for _, validationError := range errors.Errors {
		s += validationError.ErrorMessage + "\n"
		for k, v := range validationError.Fields {
			s += " - " + k + ": " + v + ".\n"
		}
	}
	return s
}

func addMatchers() {
	var xmlType = filetype.NewType("xml", "application/xml")
	filetype.NewMatcher(xmlType, func(buf []byte) bool {
		return len(buf) > 1 && buf[0] == 0x3c && buf[1] == 0x3f
	})
}
