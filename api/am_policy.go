package api

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"os"
	"bytes"

	"github.com/davecgh/go-spew/spew"
	"github.com/ghodss/yaml"
	"github.com/forgerock/frconfig/crest"
	"github.com/golang/glog"
)

// Policy in OpenAMConnection
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
func ListPolicy(openam *OpenAMConnection) ([]Policy, error) {

	client := &http.Client{}
	req := openam.newRequest("GET", "/json/policies?_queryFilter=true",nil)

	dump, err := httputil.DumpRequestOut(req, true)

	fmt.Printf("dump req is %s", dump)

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
		glog.Errorf("cant get result type", err)
	}

	//fmt.Printf("result = %s", result)

	spew.Dump(result)

	return result.Result, err
}

func PolicytoYAML(policies []Policy) {

	for _, p := range policies {
		s, err := json.Marshal(p)
		if err != nil {
			fmt.Println("error %v", err)
		} else {

			fmt.Println("json:", string(s))
		}

		y,err := yaml.Marshal(p)
		if err != nil {
			fmt.Println("error %v", err)
		} else {

			fmt.Println("yaml:\n", string(y))
		}

	}

}

// Export all the policies as a XACML policy set
func (openam *OpenAMConnection) ExportXacmlPolicies() (string, error) {
	req := openam.newRequest("GET", "/xacml/policies",nil)

	client := &http.Client{}

	resp, err := client.Do(req)
	defer resp.Body.Close()

	if err != nil {
		return "", err
	}

	body, err := ioutil.ReadAll(resp.Body)

	return string(body),err

}

// Export all the policies as a JSON or YAML policy set string
func (openam *OpenAMConnection) ExportPolicies(format,realm string) (out string, err error) {
	url := fmt.Sprintf("/json%s/policies?_queryFilter=true", realm)
	req := openam.newRequest("GET", url,nil)

	result,err := crest.GetCRESTResult(req)
	if err != nil {
		fmt.Errorf("Could not get policies, err=%v",err)
		return "",err
	}

	fmt.Printf("Crest result = %+v", result)

	var m  = make(map[string]string)

	if realm != "" {
		m["realm"] = realm
	}

	var obj = &crest.FRObject{POLICY, m, &result.Result}

	return obj.Marshal(format)

}

type  PolicyArray  []interface{}

func (openam *OpenAMConnection) ImportPoliciesFromFile(filePath string)  error {
	f,err := os.Open(filePath)
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

	var p PolicyArray

	err = json.Unmarshal(bytes, &p)

	if err != nil {
		glog.Errorf("Can't unmarshal json file, Err=%v", err)
	}

	return err

}

// Create Policies in OpenAM instance. If continueOnError is true, keep trying
// to create policies even if a single create fails.  If overWrite is true,
// First delete the policy and then create it
func (am *OpenAMConnection) CreatePolicies(obj *crest.FRObject, overWrite, continueOnError bool) (err error) {
	// each item is a policy

	for _,v := range *obj.Items {

		// bytes,err :=  json.Marshal(v)

		// cast to map so we can look at policy attrs
		m :=  v.(map[string]interface{})


		realm,_  := (*obj).Metadata["realm"]

		//fmt.Printf("Creating Policy %v realm = %s ", m, realm)

		e := am.CreatePolicy(m,overWrite, realm)
		if e != nil {
			if  ! continueOnError {
				return e
			}
			err = e
		}

	}
	return err
}

// Create a single policy described by the json
func (am *OpenAMConnection) CreatePolicy(p map[string]interface{} , overWrite bool, realm string) (err error) {
	//crest.

	if  overWrite { // try to delete existing policy if it exists
		policyName := p["name"].(string)
		err = am.DeletePolicy(policyName,realm)
		if err != nil {
			fmt.Printf("Warning - can't delete policy! err=%v", err)
		}
	}
	json,err := json.Marshal(p)
	r := bytes.NewReader(json)
	url := fmt.Sprintf("/json%s/policies?_action=create", realm)
	req :=  am.newRequest("POST", url , r)

	//req.

	_,err = crest.GetCRESTResult(req)

	//fmt.Printf("create policy result = %v err= %v", result, err)
	return

}


// Delete the named policy. If the policy does exist, we do not return an error code
func (am *OpenAMConnection)DeletePolicy(name, realm string) (err error) {
	url := fmt.Sprintf("/json%s/policies/%s", realm, name)

	req := am.newRequest("DELETE", url, nil)

	//fmt.Printf("Delete request %s\n", url)

	client := &http.Client{}

	resp, err := client.Do(req)

	if err != nil {
		return
	}

	defer resp.Body.Close()

	//fmt.Printf("code = %d stat = %v", resp.StatusCode, resp.Status)

	if resp.StatusCode != 404 && resp.StatusCode != 200 {
		err = fmt.Errorf("Error deleting resource %s, err=", name, resp.Status)
	}
	return
}
// Script query - to get Uuid
// http://openam.test.com:8080/openam/json/scripts?_pageSize=20&_sortKeys=name&_queryFilter=true&_pagedResultsOffset=0