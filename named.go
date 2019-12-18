package main

import (
	"flag"
	"github.com/golang/glog"
	"github.com/nais/named/api"
	"net/http"
)

const port string = ":8081"

func main() {
	fasitURL := flag.String("fasitUrl", "https://fasit.example.no", "URL to fasit instance")
	clusterName := flag.String("clusterName", "dev-fss", "NAIS cluster name")
	flag.Parse()

	api := api.NewAPI(*fasitURL, *clusterName)

	glog.Infof("Named running on port %s using fasit instance %s", port, *fasitURL)

	err := http.ListenAndServe(port, api.MakeHandler())
	if err != nil {
		panic(err)
	}
}
