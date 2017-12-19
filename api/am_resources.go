package api

import (
	"fmt"
	"github.com/forgerock/frconfig/crest"
	"net/http"
	"io/ioutil"
	"encoding/json"
	"github.com/davecgh/go-spew/spew"
	"github.com/golang/glog"
)

const (
	POLICY = "am.policy"
)

type ResourceType struct {
	UUID				string 		`json:"uuid"`
	Name 				string 		`json:"name"`
	Description 		string 		`json:"description"`
	Patterns 			[]string 	`json:"patterns"`
	Actions				interface{} `json:"actions"`
	CreatedBy			string 		`json:"createdBy"`
	CreationDate		int64 		`json:"creationDate"`
	LastModifiedBy		string 		`json:"lastModifiedBy"`
	LastModifiedDate 	int64 		`json:"lastModifiedBy"`
}

type ResourceTypeResult struct {
	Result             		[]ResourceType `json:"result"`
	ResultCount        		int64          `json:"resultCount"`
	PagedResultsCookie 		string         `json:"pagedResultsCookie"`
	RemainingPagedResults   int64          `json:"remainingPagedResults"`
}

func init() {
	crest.RegisterCreateObjectHandler([]string{POLICY}, CreateObjects)
}

func GetOpenAMUser() string {
	return "user"
}

func GetOpenAMPassword() string {
	return "pass"
}

func GetOpenAMUrl() string {
	return "url"
}

func CreateObjects(obj *crest.FRObject, overwrite, continueOnError bool) (err error) {
	am, err := GetOpenAMConnection()
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

func (am *OpenAMConnection) ListResourceTypes() ([]ResourceType, error) {
	client := &http.Client{}
	request := am.newRequest("GET", "/json/resourcetypes?_queryFilter=true", nil)
	//dump, err := httputil.DumpRequestOut(request, true)

	response, err := client.Do(request)
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
