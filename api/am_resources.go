package api

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/forgerock/frconfig/crest"
	"github.com/golang/glog"
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

func createObjects(obj *crest.FRObject, overwrite, continueOnError bool) (err error) {
	am, err := GetAmConnection(&IssoResource{})
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
	request, err := am.createNewRequest("GET", "/json/resourcetypes?_queryFilter=true", nil)
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

	return result.Result, err
}
