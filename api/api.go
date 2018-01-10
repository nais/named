package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/golang/glog"
	ver "github.com/nais/named/api/version"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"goji.io"
	"goji.io/pat"
	"golang.org/x/crypto/ssh"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
	//"github.com/forgerock/frconfig/amconfig"
)

// Api contains fasit instance and cluster to fetch AM information from
type Api struct {
	FasitUrl    string
	ClusterName string
}

// NamedConfigurationRequest contains the information of the application to configure in AM
type NamedConfigurationRequest struct {
	Application string `json:"application"`
	Version     string `json:"version"`
	Environment string `json:"environment"`
	Zone        string `json:"zone"`
	Username    string `json:"username"`
	Password    string `json:"password"`
}

// AppError contains error and response code
type AppError interface {
	error
	Code() int
}

type appError struct {
	OriginalError error
	Message       string
	StatusCode    int
}

const sshPort = "22"

// NewApi initializes fasit instance information
func NewApi(fasitUrl, clusterName string) *Api {
	return &Api{
		FasitUrl:    fasitUrl,
		ClusterName: clusterName,
	}
}

func (e appError) Code() int {
	return e.StatusCode
}
func (e appError) Error() string {
	if e.OriginalError != nil {
		return fmt.Sprintf("%s: %s (%d)", e.Message, e.OriginalError.Error(), e.StatusCode)
	}
	return fmt.Sprintf("%s (%d)", e.Message, e.StatusCode)

}

type appHandler func(w http.ResponseWriter, r *http.Request) *appError

func (fn appHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if e := fn(w, r); e != nil { // e is *appError, not os.Error.
		http.Error(w, e.Error(), e.StatusCode)
	}
}

var (
	requests = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "requests", Help: "requests pr path"}, []string{"path"},
	)
	configurations = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "configurations", Help: "configurations done by nameD"}, []string{"nameD"},
	)
)

func init() {
	prometheus.MustRegister(requests)
	prometheus.MustRegister(configurations)
}

// MakeHandler creates REST endpoint handlers
func (api Api) MakeHandler() http.Handler {
	mux := goji.NewMux()
	mux.Handle(pat.Get("/isalive"), appHandler(api.isAlive))
	mux.Handle(pat.Get("/metrics"), promhttp.Handler())
	mux.Handle(pat.Get("/version"), appHandler(api.version))
	mux.Handle(pat.Post("/configure"), appHandler(api.configure))
	return mux
}

func (api Api) isAlive(w http.ResponseWriter, _ *http.Request) *appError {
	requests.With(prometheus.Labels{"path": "isalive"}).Inc()
	fmt.Fprint(w, "")
	return nil
}

func (api Api) version(w http.ResponseWriter, _ *http.Request) *appError {
	response := map[string]string{"version": ver.Version, "revision": ver.Revision}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		return &appError{err, "Unable to encode JSON", 500}
	}

	return nil
}

func (api Api) configure(w http.ResponseWriter, r *http.Request) *appError {
	requests.With(prometheus.Labels{"path": "configure"}).Inc()
	namedConfigurationRequest, err := unmarshalConfigurationRequest(r.Body)

	if err != nil {
		return &appError{err, "Unable to unmarshal configuration namedConfigurationRequest", http.StatusBadRequest}
	}

	fasitClient := FasitClient{api.FasitUrl, namedConfigurationRequest.Username, namedConfigurationRequest.Password}

	if "sbs" == strings.ToLower(namedConfigurationRequest.Zone) {
		resourceRequest := createResourceRequest("OpenAM", "OpenAM")
		openamResource, error := fasitClient.GetOpenAmResource(resourceRequest, namedConfigurationRequest.Environment, namedConfigurationRequest.Application,
			namedConfigurationRequest.Zone)
		if error != nil {
			glog.Errorf("Could not get OpenAM resource %s", error)
			return &appError{error, "Fasit OpenAM resource unavailable", http.StatusNotFound}
		}

		files, err := GenerateAmFiles(namedConfigurationRequest)
		if err != nil {
			glog.Errorf("Could not download am policy files: %s", err)
			return &appError{err, "Policy files not found", http.StatusNotFound}
		}

		sshClient, sshSession, err := SshConnect(openamResource.Username, openamResource.Password, openamResource.Hostname, sshPort)
		if err != nil {
			glog.Errorf("Could not get ssh session on %s %s", openamResource.Hostname, err)
			return &appError{err, "SSH session failed", http.StatusBadRequest}
		}
		defer sshSession.Close()
		defer sshClient.Close()

		err = CopyFilesToAmServer(sshClient, files, namedConfigurationRequest.Application)
		if err != nil {
			glog.Errorf("Could not to copy files to AM server %s", err)
			return &appError{err, "AM policy files transfer failed", http.StatusBadRequest}
		}

		configurations.With(prometheus.Labels{"nameD": namedConfigurationRequest.Application}).Inc()
		//JobQueue <- Job{Api: api}
		if err := api.runAmPolicyScript(namedConfigurationRequest, sshSession); err != nil {
			glog.Errorf("Failed to run script: %s", err)
			return &appError{err, "AM policy script failed", http.StatusBadRequest}
		}

		w.Write([]byte("AM policy configured for " + namedConfigurationRequest.Application + " in " +
			namedConfigurationRequest.Environment))

	} else if "fss" == strings.ToLower(namedConfigurationRequest.Zone) {
		issoResource, err := fasitClient.GetIssoResource(namedConfigurationRequest.Environment,
			namedConfigurationRequest.Application, namedConfigurationRequest.Zone)
		if err != nil {
			glog.Errorf("Could not get OIDC resource %s", err)
			return &appError{err, "Fasit OIDC resource unavailable", http.StatusNotFound}
		}

		am, error := GetAmConnection(issoResource)
		if error != nil {
			glog.Errorf("Failed to connect to AM server: %s", error)
			return &appError{error, "AM server connection failed", http.StatusServiceUnavailable}
		}

		glog.Infof(am.BaseURL, am.User)


		w.Write([]byte("OIDC configured for " + namedConfigurationRequest.Application + " in " +
			namedConfigurationRequest.Environment))
	} else {
		return &appError{errors.New("No AM configurations available for this zone"), "Zone has to be fss or sbs, not " + namedConfigurationRequest.Zone,
			http.StatusBadRequest}
	}

	return nil
}

func (api Api) runAmPolicyScript(request NamedConfigurationRequest, sshSession *ssh.Session) error {
	cmd := fmt.Sprintf("sudo python /opt/openam/scripts/openam_policy.py %s %s", request.Application, request.Application)

	modes := ssh.TerminalModes{
		ssh.ECHO: 0, // Disable echoing
	}

	if err := sshSession.RequestPty("xterm", 80, 40, modes); err != nil {
		glog.Infof("Could not set pty")
	}

	var stdoutBuf bytes.Buffer

	sshSession.Stdout = &stdoutBuf

	glog.Infof("Running command %s", cmd)
	err := sshSession.Run(cmd)
	if err != nil {
		return fmt.Errorf("Could not run command %s %s", cmd, err)
	}
	glog.Infof("AM policy updated for %s in environment %s", request.Application,
		request.Environment)
	return nil
}

// Validate performs validation of NamedConfigurationRequest
func (r NamedConfigurationRequest) Validate() []error {
	required := map[string]*string{
		"Application": &r.Application,
		"Version":     &r.Version,
		"Environment": &r.Environment,
		"Zone":        &r.Zone,
		"Username":    &r.Username,
		"Password":    &r.Password,
	}

	var errs []error
	for key, pointer := range required {
		if len(*pointer) == 0 {
			errs = append(errs, fmt.Errorf("%s is required and is empty", key))
		}
	}

	if r.Zone != "fss" && r.Zone != "sbs" && r.Zone != "iapp" {
		errs = append(errs, errors.New("Zone can only be fss, sbs or iapp"))
	}

	return errs
}

func unmarshalConfigurationRequest(body io.ReadCloser) (NamedConfigurationRequest, error) {
	requestBody, err := ioutil.ReadAll(body)
	if err != nil {
		return NamedConfigurationRequest{}, fmt.Errorf("Could not read configuration request body %s", err)
	}

	var request NamedConfigurationRequest
	if err = json.Unmarshal(requestBody, &request); err != nil {
		return NamedConfigurationRequest{}, fmt.Errorf("Could not unmarshal body %s", err)
	}

	return request, nil
}

func createResourceRequest(alias, resourceType string) ResourceRequest {
	return ResourceRequest{
		Alias:        alias,
		ResourceType: resourceType,
	}
}
