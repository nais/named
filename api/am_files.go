package api

import (
	"fmt"
	"github.com/golang/glog"
	"github.com/h2non/filetype"
	"golang.org/x/crypto/ssh"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
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
func GenerateAmFiles(request NamedConfigurationRequest) ([]string, error) {
	policyFiles, err := downloadPolicies(request)
	if err != nil {
		glog.Errorf("Could not download policy files %s", err)
		return []string{}, err
	}

	validationErrors := ValidatePolicyFiles(policyFiles)
	if len(validationErrors.Errors) != 0 {
		return []string{}, validationErrors
	}

	return policyFiles, nil
}

func downloadPolicies(request NamedConfigurationRequest) ([]string, error) {
	urls := createPolicyFileUrls(request.Application, request.Version)
	files, err := fetchPolicyFiles(urls, request.Application)
	if err != nil {
		glog.Errorf("Could not fetch policy files: %s", err)
	}

	return files, nil
}

func createPolicyFileUrls(application, version string) []string {
	var urls = []string{}
	baseUrl := "https://repo.adeo.no/repositories/raw/nais"
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
			return []string{}, fmt.Errorf("Got HTTP status code %d fetching manifest from URL: %s", response.StatusCode,
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
		return fmt.Errorf("Could not transfer files to AM server: %s", err)
	}

	for _, policyFile := range policyFiles {
		srcFile, err := os.Open(policyFile)
		if err != nil {
			return fmt.Errorf("Could not open file %s: %s", policyFile, err)
		}

		srcFileInfo, err := srcFile.Stat()
		if err != nil {
			return fmt.Errorf("Could not stat file %s: %s", policyFile, err)
		}

		defer srcFile.Close()

		_ = sftpClient.Mkdir("/tmp/" + application)
		destFile, err := sftpClient.Create(policyFile)
		if err != nil {
			return fmt.Errorf("Could not create am file %s: %s", policyFile, err)
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

func cleanupLocalFiles(policyFiles []string) error {
	for _, fileName := range policyFiles {
		err := os.Remove(fileName)
		if err != nil {
			return fmt.Errorf("Could not remove file: %s", fileName)
		}
	}
	return nil
}
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

	if kind.Extension == "txt" {
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

	var txtType = filetype.NewType("txt", "application/text")
	filetype.NewMatcher(txtType, func(buf []byte) bool {
		return len(buf) > 1 && buf[0] == 0x74 && buf[1] == 0x65
	})
}
