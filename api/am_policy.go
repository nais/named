package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"os"

	"github.com/forgerock/frconfig/crest"
	"github.com/ghodss/yaml"
	"github.com/golang/glog"
)

// Policy in AMConnection
type Policy struct {
	Name             string      `json:"name"`
	Active           bool        `json:"active"`
	ApplicationName  string      `json:"applicationName"`
	ActionValues     interface{} `json:"actionValues"`
	Resources        []string    `json:"resources"`
	Description      string      `json:"description"`
	Subject          interface{} `json:"subject"`
	Condition        interface{} `json:"condition"`
	ResourceTypeUUID string      `json:"resourceTypeUuid"`
	CreatedBy        string      `json:"createdBy"`
	CreationDate     string      `json:"creationDate"`
	LastModifiedBy   string      `json:"lastModifiedBy"`
	LastModifiedDate string      `json:"lastModifiedDate"`
}

// A PolicyResultList is a set of Policies
type PolicyResultList struct {
	Result                []Policy `json:"result"`
	ResultCount           int64    `json:"resultCount"`
	PagedResultsCookie    string   `json:"pagedResultsCookie"`
	RemainingPagedResults int64    `json:"remainingPagedResults"`
}

// ListPolicy lists all OpenAM policies for a realm
func ListPolicy(am *AMConnection) ([]Policy, error) {

	client := &http.Client{}
	req, err := am.createNewRequest("GET", "/json/policies?_queryFilter=true", nil)
	if err != nil {
		glog.Errorf("Could not create request: %s", err)
	}

	dump, _ := httputil.DumpRequestOut(req, true)
	glog.Infof("dump req is %s", dump)

	//debug(httputil.DumpResponse(response, true))

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	body, _ := ioutil.ReadAll(resp.Body)

	//fmt.Println("response Body:", string(body))

	var result PolicyResultList

	err = json.Unmarshal(body, &result)

	if err != nil {
		glog.Errorf("Could not get result type: %s", err)
	}

	return result.Result, err
}

func policytoYAML(policies []Policy) {
	for _, p := range policies {
		s, err := json.Marshal(p)
		if err != nil {
			glog.Infof("Error %v", err)
		} else {
			glog.Infof("json: %s", string(s))
		}

		y, err := yaml.Marshal(p)
		if err != nil {
			glog.Infof("error %v", err)
		} else {
			glog.Infof("yaml: %s", string(y))
		}

	}

}

// ExportXacmlPolicies exports all the policies as a XACML policy set
func (am *AMConnection) ExportXacmlPolicies() (string, error) {
	req, err := am.createNewRequest("GET", "/xacml/policies", nil)
	if err != nil {
		glog.Errorf("Could not create request: %s", err)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		glog.Errorf("Could not execute request: %s", err)
	}

	defer resp.Body.Close()

	if err != nil {
		glog.Errorf("Could not get response: %s", err)
		return "", err
	}

	body, err := ioutil.ReadAll(resp.Body)

	return string(body), err

}

// ExportPolicies exports all the policies as a JSON or YAML policy set string
func (am *AMConnection) ExportPolicies(format, realm string) (out string, err error) {
	url := fmt.Sprintf("/json/policies?realm=%s&_queryFilter=true", realm)
	req, err := am.createNewRequest("GET", url, nil)

	result, err := crest.GetCRESTResult(req)
	if err != nil {
		glog.Errorf("Could not get policies, err=%v", err)
		return "", err
	}

	glog.Infof("Crest result = %+v", result)

	var m = make(map[string]string)

	if realm != "" {
		m["realm"] = realm
	}

	var obj = &crest.FRObject{POLICY, m, &result.Result}

	return obj.Marshal(format)

}

type policyArrays []interface{}

func (am *AMConnection) importPoliciesFromFile(filePath string) error {
	f, err := os.Open(filePath)
	defer f.Close()
	if err != nil {
		glog.Errorf("Can't open file %v, err=%v", filePath, err)
	}
	//r := bufio.NewReader(f)

	bytes, err := ioutil.ReadAll(f)

	if err != nil {
		glog.Errorf("Can't read policy file. Err = %v", err)
		return err
	}

	var p policyArrays

	err = json.Unmarshal(bytes, &p)

	if err != nil {
		glog.Errorf("Can't unmarshal json file, Err=%v", err)
	}

	return err

}

// CreatePolicies creates policies in AM instance. If continueOnError is true, keep trying
// to create policies even if a single create fails.  If overWrite is true,
// First delete the policy and then create it
func (am *AMConnection) CreatePolicies(obj *crest.FRObject, overWrite, continueOnError bool) (err error) {
	// each item is a policy

	for _, v := range *obj.Items {

		// bytes,err :=  json.Marshal(v)

		// cast to map so we can look at policy attrs
		m := v.(map[string]interface{})

		realm, _ := (*obj).Metadata["realm"]

		//glog.Infof("Creating Policy %v realm = %s ", m, realm)

		e := am.CreatePolicy(m, overWrite, realm)
		if e != nil {
			if !continueOnError {
				return e
			}
			err = e
		}

	}
	return err
}

// CreatePolicy creates a single policy described by the json
func (am *AMConnection) CreatePolicy(p map[string]interface{}, overWrite bool, realm string) (err error) {
	if overWrite { // try to delete existing policy if it exists
		policyName := p["name"].(string)
		err = am.DeletePolicy(policyName, realm)
		if err != nil {
			glog.Infof("Warning - can't delete policy! err=%v", err)
		}
	}
	json, err := json.Marshal(p)
	r := bytes.NewReader(json)
	url := fmt.Sprintf("/json%s/policies?_action=create", realm)
	req, err := am.createNewRequest("POST", url, r)
	if err != nil {
		glog.Errorf("Could not create request: %s", err)
	}

	_, err = crest.GetCRESTResult(req)
	if err != nil {
		return err
	}

	return
}

// DeletePolicy erases the named policy. If the policy does exist, we do not return an error code
func (am *AMConnection) DeletePolicy(name, realm string) (err error) {
	url := fmt.Sprintf("/json%s/policies/%s", realm, name)

	req, err := am.createNewRequest("DELETE", url, nil)
	if err != nil {
		return err
	}

	//glog.Infof("Delete request %s\n", url)

	client := &http.Client{}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	//glog.Infof("code = %d stat = %v", resp.StatusCode, resp.Status)

	if resp.StatusCode != 404 && resp.StatusCode != 200 {
		err = fmt.Errorf("Error deleting resource %s, err=%s", name, resp.Status)
	}
	return
}

// Script query - to get Uuid
// http://openam.test.com:8080/openam/json/scripts?_pageSize=20&_sortKeys=name&_queryFilter=true&_pagedResultsOffset=0
