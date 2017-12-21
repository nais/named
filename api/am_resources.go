package api

import (
	"encoding/json"
	"fmt"
	"github.com/davecgh/go-spew/spew"
	"github.com/forgerock/frconfig/crest"
	"github.com/golang/glog"
	"io/ioutil"
	"net/http"
)

// POLICY sets the policy name on AM server
const POLICY = "am.policy"

// ResourceType contains the AM resource type
type ResourceType struct {
	UUID             string      `json:"uuid"`
	Name             string      `json:"name"`
	Description      string      `json:"description"`
	Patterns         []string    `json:"patterns"`
	Actions          interface{} `json:"actions"`
	CreatedBy        string      `json:"createdBy"`
	CreationDate     int64       `json:"creationDate"`
	LastModifiedBy   string      `json:"lastModifiedBy"`
	LastModifiedDate int64       `json:"lastModifiedDate"`
}

// ResourceTypeResult contains the AM result values when fetching resources
type ResourceTypeResult struct {
	Result                []ResourceType `json:"result"`
	ResultCount           int64          `json:"resultCount"`
	PagedResultsCookie    string         `json:"pagedResultsCookie"`
	RemainingPagedResults int64          `json:"remainingPagedResults"`
}

func init() {
	crest.RegisterCreateObjectHandler([]string{POLICY}, createObjects)
}

// GetAmUser returns username for AM connection
func GetAmUser() string {
	return "user"
}

// GetAmPassword returns password for AM connection
func GetAmPassword() string {
	return "pass"
}

// GetAmUrl returns base URL for AM connection
func GetAmUrl() string {
	return "url"
}

func createObjects(obj *crest.FRObject, overwrite, continueOnError bool) (err error) {
	am, err := GetAmConnection()
	if err != nil {
		return err
	}

	switch obj.Kind {
	case POLICY:
		err = am.CreatePolicies(obj, overwrite, continueOnError)
	default:
		err = fmt.Errorf("Unknown object type %s", obj.Kind)
	}

	return
}

// ListResourceTypes returns the available resource types from the AM server
func (am *AMConnection) ListResourceTypes() ([]ResourceType, error) {
	client := &http.Client{}
	request, err := am.newRequest("GET", "/json/resourcetypes?_queryFilter=true", nil)
	//dump, err := httputil.DumpRequestOut(request, true)
	if err != nil {
		glog.Errorf("Failed to create request: %s", err)
	}

	response, err := client.Do(request)
	if err != nil {
		glog.Errorf("Could not execute request: %s", err)
	}

	defer response.Body.Close()

	if err != nil {
		return nil, err
	}

	body, _ := ioutil.ReadAll(response.Body)
	var result ResourceTypeResult
	err = json.Unmarshal(body, &result)

	if err != nil {
		glog.Errorf("Can not get result type: %s", err)
	}

	spew.Dump(result)

	return result.Result, err
}