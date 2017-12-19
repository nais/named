package api

import (
	"fmt"

	"github.com/forgerock/frconfig/crest"

	"net/http"
	"io/ioutil"
	"encoding/json"
	"log"
	"github.com/davecgh/go-spew/spew"
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
	return "amadmin"
}

func GetOpenAMPassword() string {
	return "221234567890"
}

func GetOpenAMUrl() string {
	return "https://isso-drift.adeo.no/isso"
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

func (openam *OpenAMConnection) ListResourceTypes() ([]ResourceType, error) {
	client := &http.Client{}
	request := openam.newRequest("GET", "/json/resourcetypes?_queryFilter=true", nil)
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
		log.Fatalf("Can not get result type", err)
	}

	spew.Dump(result)

	return result.Result, err
}
