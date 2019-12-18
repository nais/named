package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"time"

	"github.com/nais/named/api"
	"github.com/spf13/cobra"
)

const configureEndpoint = "/configure"
const defaultCluster = "preprod-fss"

var clustersDict = map[string]string{
	"nais-dev":     "nais.devillo.no",
	"dev-fss":      "nais.preprod.local",
	"prod-fss":     "nais.adeo.no",
	"preprod-iapp": "nais-iapp.preprod.local",
	"prod-iapp":    "nais-iapp.adeo.no",
	"dev-sbs":      "nais.oera-q.local",
	"prod-sbs":     "nais.oera.no",
}

func validateCluster(cluster string) (string, error) {
	url, exists := clustersDict[cluster]
	if exists {
		return url, nil
	}

	errmsg := fmt.Sprint("Cluster is not valid, please choose one of: ")
	for key := range clustersDict {
		errmsg = errmsg + fmt.Sprintf("%s, ", key)
	}

	return "", errors.New(errmsg)
}

func getClusterUrl(cluster string) (string, error) {
	if len(cluster) == 0 {
		cluster = defaultCluster
	}

	url, err := validateCluster(cluster)
	if err != nil {
		return "", err
	}

	return "https://named." + url, nil
}

var configurationCmd = &cobra.Command{
	Use:   "configure",
	Short: "Configures your application in AM",
	Long:  `Configures your application in AM`,
	Run: func(cmd *cobra.Command, args []string) {
		configurationRequest := api.NamedConfigurationRequest{
			Username: os.Getenv("NAIS_USERNAME"),
			Password: os.Getenv("NAIS_PASSWORD"),
		}

		var cluster string
		strings := map[string]*string{
			"app":      &configurationRequest.Application,
			"version":  &configurationRequest.Version,
			"env":      &configurationRequest.Environment,
			"username": &configurationRequest.Username,
			"password": &configurationRequest.Password,
			"cluster":  &cluster,
		}

		zone := api.GetZone(cluster)

		for key, pointer := range strings {
			if value, err := cmd.Flags().GetString(key); err != nil {
				fmt.Printf("Error when getting flag: %s. %v\n", key, err)
				os.Exit(1)
			} else if len(value) > 0 {
				*pointer = value
			}
		}

		if api.ZoneFss == zone {
			if value, err := cmd.Flags().GetStringArray("contexts"); err != nil {
				fmt.Printf("Flag --contexts/-c not defined")
				os.Exit(1)
			} else {
				configurationRequest.ContextRoots = value
			}
		}

		if err := configurationRequest.Validate(zone); err != nil {
			fmt.Printf("Configuration request is not valid: %v\n", err)
			os.Exit(1)
		}

		clusterUrl, err := getClusterUrl(cluster)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}

		jsonStr, err := json.Marshal(configurationRequest)
		if err != nil {
			fmt.Printf("Error while marshalling JSON: %v\n", err)
			os.Exit(1)
		}

		start := time.Now()

		resp, err := http.Post(clusterUrl+configureEndpoint, "application/json", bytes.NewBuffer(jsonStr))
		if err != nil {
			fmt.Printf("Error while POSTing to API: %v\n", err)
			os.Exit(1)
		}

		defer resp.Body.Close()

		body, _ := ioutil.ReadAll(resp.Body)
		fmt.Println("response Status:", resp.Status)
		fmt.Println("response Body:", string(body))

		if resp.StatusCode > 299 {
			os.Exit(1)
		}

		elapsed := time.Since(start)
		fmt.Printf("Configuration successful, took %v\n", elapsed)
	},
}

func init() {
	RootCmd.AddCommand(configurationCmd)
	configurationCmd.Flags().StringP("app", "a", "", "name of your app")
	configurationCmd.Flags().StringP("version", "v", "", "version you want to configure for")
	configurationCmd.Flags().StringP("cluster", "c", "", "the cluster you want to deploy to")
	configurationCmd.Flags().StringP("env", "e", "", "environment you want to use")
	configurationCmd.Flags().StringP("contexts", "r", "", "the context roots to configure in ISSO")
	configurationCmd.Flags().StringP("username", "u", "", "the username")
	configurationCmd.Flags().StringP("password", "p", "", "the password")
	configurationCmd.Flags().Bool("wait", false, "whether to wait until the deploy has succeeded (or failed)")
}
