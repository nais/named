package api

import (
	"testing"
	"fmt"
)

var baseUrl = "https://server.domain.com"
var amc = OpenAMConnection{BaseURL:baseUrl, User:"user", Password:"pass"}

func TestRest(t *testing.T) {

	err := amc.Authenticate()
	if err != nil {
		fmt.Errorf("Could not authenticate on OpenAM server %s", baseUrl )
	}

	rt, err := amc.ListResourceTypes()

	if err != nil {
		fmt.Errorf("Could not list resource types")
	}

	for _, v := range rt {
		fmt.Printf("%v", v)
	}
}

func TestJsonExport(t *testing.T) {
	if err := amc.Authenticate(); err != nil {
		fmt.Errorf("Could not authenticate on OpenAM server %s", baseUrl )
	}

	json, err := amc.ExportPolicies("json", "%2F")

	if err != nil {
		fmt.Errorf("Could not list resource types")
	}

	fmt.Printf(json)
}