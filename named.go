package main

import (
	"flag"
	"github.com/golang/glog"
	"github.com/nais/named/api"
	"net/http"
)

const Port string = ":8081"

func main() {
	fasitUrl := flag.String("fasitUrl", "https://fasit.example.no", "URL to fasit instance")
	clusterName := flag.String("clusterName", "preprod-fss", "NAIS cluster name")
	flag.Parse()

	api := api.NewApi(*fasitUrl, *clusterName)

	glog.Infof("Named running on port %s using fasit instance %s", Port, *fasitUrl)

	err := http.ListenAndServe(Port, api.MakeHandler())
	if err != nil {
		panic(err)
	}
}
