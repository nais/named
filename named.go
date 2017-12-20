package main

import (
	"flag"
	"github.com/golang/glog"
	"github.com/nais/named/api"
	"net/http"
)

const port string = ":8081"

func main() {
	fasitUrl := flag.String("fasitUrl", "https://fasit.example.no", "URL to fasit instance")
	clusterName := flag.String("clusterName", "preprod-fss", "NAIS cluster name")
	flag.Parse()

	api := api.NewApi(*fasitUrl, *clusterName)

	glog.Infof("Named running on port %s using fasit instance %s", port, *fasitUrl)

	err := http.ListenAndServe(port, api.MakeHandler())
	if err != nil {
		panic(err)
	}
}
