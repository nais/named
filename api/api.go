package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/golang/glog"
	ver "github.com/nais/named/api/version"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"goji.io"
	"goji.io/pat"
	"golang.org/x/crypto/ssh"
	//"github.com/forgerock/frconfig/amconfig"
)

// API contains fasit instance and cluster to fetch AM information from
type API struct {
	FasitURL    string
	ClusterName string
}

// NamedConfigurationRequest contains the information of the application to configure in AM
type NamedConfigurationRequest struct {
	Application     string   `json:"application"`
	Version         string   `json:"version"`
	Environment     string   `json:"environment"`
	Username        string   `json:"username"`
	Password        string   `json:"password"`
	ContextRoots    []string `json:"contextroots"`
	RedirectionUris []string
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

const (
	sshPort           = "22"
	// ZoneFss is secure zone
	ZoneFss           = "fss"
	// ZoneSbs is or outer zone
	ZoneSbs           = "sbs"
	clusterPreprodSbs = "preprod-sbs"
	clusterPreprodFss = "preprod-fss"
	clusterProdSbs    = "prod-sbs"
	clusterProdFss    = "prod-fss"
)

// NewAPI initializes fasit instance information
func NewAPI(fasitURL, clusterName string) *API {
	return &API{
		FasitURL:    fasitURL,
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
		prometheus.CounterOpts{Name: "configurations", Help: "configurations done by nameD"}, []string{"named_app"},
	)
)

func init() {
	prometheus.MustRegister(requests)
	prometheus.MustRegister(configurations)
}

// MakeHandler creates REST endpoint handlers
func (api *API) MakeHandler() http.Handler {
	mux := goji.NewMux()
	mux.Handle(pat.Get("/isalive"), appHandler(api.isAlive))
	mux.Handle(pat.Get("/metrics"), promhttp.Handler())
	mux.Handle(pat.Get("/version"), appHandler(api.version))
	mux.Handle(pat.Post("/configure"), appHandler(api.configure))
	return mux
}

func (api *API) isAlive(w http.ResponseWriter, _ *http.Request) *appError {
	requests.With(prometheus.Labels{"path": "isalive"}).Inc()
	fmt.Fprint(w, "")
	return nil
}

func (api *API) version(w http.ResponseWriter, _ *http.Request) *appError {
	response := map[string]string{"version": ver.Version, "revision": ver.Revision}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		return &appError{err, "Unable to encode JSON", 500}
	}

	return nil
}

func (api *API) configure(w http.ResponseWriter, r *http.Request) *appError {
	requests.With(prometheus.Labels{"path": "configure"}).Inc()

	namedConfigurationRequest, err := unmarshalConfigurationRequest(r.Body)
	if err != nil {
		return &appError{err, "Unable to unmarshal configuration namedConfigurationRequest", http.StatusBadRequest}
	}

	fasitClient := FasitClient{api.FasitURL, namedConfigurationRequest.Username, namedConfigurationRequest.Password}
	fasitErr := validateFasitRequirements(&fasitClient, &namedConfigurationRequest)
	if fasitErr != nil {
		return fasitErr
	}

	zone := GetZone(api.ClusterName)

	if errs := namedConfigurationRequest.Validate(zone); errs != nil {
		var errorString = "Configuration request is invalid: "
		for _, err := range errs {
			errorString = errorString + err.Error() + ","
		}
		return &appError{nil, errorString, http.StatusBadRequest}
	}

	if ZoneSbs == zone {
		w.Write([]byte("Configuring AM policies in SBS\n"))

		appError := configureSBSOpenam(&fasitClient, &namedConfigurationRequest, zone)
		if appError != nil {
			return appError
		}

		w.Write([]byte("AM policy configured for " + namedConfigurationRequest.Application + " in " +
			namedConfigurationRequest.Environment))

	} else if ZoneFss == zone {
		w.Write([]byte("Configuring ISSO agent in FSS\n"))

		appError := configureFSSOpenam(&fasitClient, &namedConfigurationRequest, zone)
		if appError != nil {
			return appError
		}

		w.Write([]byte("OIDC configured for " + namedConfigurationRequest.Application + " in " +
			namedConfigurationRequest.Environment + "\nAgentName: " + namedConfigurationRequest.Application + "-" +
			namedConfigurationRequest.Environment + "\nRedirection URIs:\n\t" + strings.Join(namedConfigurationRequest.RedirectionUris,
			"\n\t")))
	} else {
		return &appError{errors.New("no AM configurations available for this zone"), "Zone has to be fss or sbs, not " + zone, http.StatusBadRequest}
	}

	return nil
}

func configureSBSOpenam(fasit *FasitClient, request *NamedConfigurationRequest, zone string) *appError {
	openamResource, apErr := fasit.GetOpenAmResource(createResourceRequest("OpenAM", "OpenAM"),
		request.Environment, request.Application, zone)
	if apErr != nil {
		glog.Errorf("Could not get OpenAM resource: %s", apErr)
		return apErr
	}

	files, err := GenerateAmFiles(request)
	if err != nil {
		glog.Errorf("Could not download am policy files: %s", err)
		return &appError{err, "Policy files not found", http.StatusNotFound}
	}

	sshClient, sshSession, err := SSHConnect(&openamResource, sshPort)
	if err != nil {
		glog.Errorf("Could not get ssh session on %s %s", openamResource.Hostname, err)
		return &appError{err, "SSH session failed", http.StatusServiceUnavailable}
	}

	defer sshSession.Close()
	defer sshClient.Close()

	err = UpdatePolicyFiles(files, request.Environment)
	if err != nil {
		glog.Errorf("Could not update policy files with correct site name %s", err)
		return &appError{err, "AM policy files could not be updated", http.StatusBadRequest}
	}

	err = CopyFilesToAmServer(sshClient, files, request.Application)
	if err != nil {
		glog.Errorf("Could not to copy files to AM server; %s", err)
		return &appError{err, "AM policy files transfer failed", http.StatusBadRequest}
	}

	configurations.With(prometheus.Labels{"named_app": request.Application}).Inc()
	//JobQueue <- Job{API: api}
	if err := runAmPolicyScript(request, sshSession); err != nil {
		glog.Errorf("Failed to run script; %s", err)
		return &appError{err, "AM policy script failed", http.StatusBadRequest}
	}

	return nil
}

func configureFSSOpenam(fasit *FasitClient, request *NamedConfigurationRequest, zone string) *appError {
	agentName := fmt.Sprintf("%s-%s", request.Application, request.Environment)

	issoResource, appErr := fasit.GetIssoResource(request, zone)
	if appErr != nil {
		glog.Errorf("Could not get OIDC resource: %s", appErr)
		return appErr
	}

	am, err := GetAmConnection(&issoResource)
	if err != nil {
		glog.Errorf("Failed to connect to AM server: %s", err)
		return &appError{err, "AM server connection failed", http.StatusServiceUnavailable}
	}

	request.RedirectionUris = CreateRedirectionUris(&issoResource, request)

	configurations.With(prometheus.Labels{"named_app": request.Application}).Inc()
	if am.AgentExists(agentName) {
		glog.Infof("Deleting agent %s before re-creating it", agentName)
		am.DeleteAgent(agentName)
	}

	glog.Infof("Creating agent %s", agentName)
	agentErr := am.CreateAgent(agentName, request.RedirectionUris, &issoResource, request)
	if agentErr != nil {
		glog.Errorf("Failed to create AM agent %s: %s", agentName, agentErr)
		return &appError{agentErr, "AM agent creation failed", http.StatusBadRequest}
	}

	return nil
}

func runAmPolicyScript(request *NamedConfigurationRequest, sshSession *ssh.Session) error {
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
		return fmt.Errorf("could not run command %s %s", cmd, err)
	}
	glog.Infof("AM policy updated for %s in environment %s", request.Application,
		request.Environment)
	return nil
}

// Validate performs validation of NamedConfigurationRequest
func (r NamedConfigurationRequest) Validate(zone string) []error {
	required := map[string]*string{
		"application": &r.Application,
		"version":     &r.Version,
		"environment": &r.Environment,
		"username":    &r.Username,
		"password":    &r.Password,
	}

	var errs []error
	for key, pointer := range required {
		if len(*pointer) == 0 {
			errs = append(errs, fmt.Errorf("%s is required but empty", key))
		}
	}

	if zone == ZoneFss {
		if len(r.ContextRoots) == 0 {
			errs = append(errs, fmt.Errorf("contextRoots are required but empty"))
		}
	}

	return errs
}

func unmarshalConfigurationRequest(body io.ReadCloser) (NamedConfigurationRequest, error) {
	requestBody, err := ioutil.ReadAll(body)
	if err != nil {
		return NamedConfigurationRequest{}, fmt.Errorf("could not read configuration request body %s", err)
	}

	var request NamedConfigurationRequest
	if err = json.Unmarshal(requestBody, &request); err != nil {
		return NamedConfigurationRequest{}, fmt.Errorf("could not unmarshal body %s", err)
	}

	return request, nil
}

func createResourceRequest(alias, resourceType string) ResourceRequest {
	return ResourceRequest{
		Alias:        alias,
		ResourceType: resourceType,
	}

}

// GetZone returns zone name for the cluster
func GetZone(clusterName string) string {
	switch clusterName {
	case clusterPreprodSbs:
		return ZoneSbs
	case clusterProdSbs:
		return ZoneSbs
	case clusterPreprodFss:
		return ZoneFss
	case clusterProdFss:
		return ZoneFss
	}

	return ""
}

func validateFasitRequirements(fasit *FasitClient, request *NamedConfigurationRequest) *appError {
	application := request.Application
	fasitEnvironment := request.Environment

	if _, err := fasit.GetFasitEnvironment(fasitEnvironment); err != nil {
		glog.Errorf("Environment '%s' does not exist in Fasit", fasitEnvironment)
		return err
	}

	if err := fasit.GetFasitApplication(request.Application); err != nil {
		glog.Errorf("Application '%s' does not exist in Fasit", application)
		return err
	}

	return nil
}
