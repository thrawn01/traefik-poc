package main

import (
	"encoding/base64"
	"encoding/json"
	"log"
	"net/http"
	"regexp"
	"strings"

	"github.com/go-chi/chi"
	"github.com/pkg/errors"
)

type EndpointSpec struct {
	ServiceName string `json:"ServiceName"`
	AuthType    string `json:"AuthType"`
	Regex       string `json:"Regex"`
	regex       *regexp.Regexp
}

const (
	authTypeAccount = "account"
	authTypeDomain  = "domain"

	accountPrivateURL = "%v/v2/flagman/accounts/private"
	accountPublicURL  = "%v/v2/flagman/accounts/public"
	domainPrivateURL  = "%v/v2/flagman/domains/$1/private"
)

func compileRegex(spec []*EndpointSpec) error {
	var err error
	for i, _ := range spec {
		spec[i].regex, err = regexp.Compile(spec[i].Regex)
		if err != nil {
			return errors.Wrapf(err, "while compiling regex '%s'", spec[i].Regex)
		}
	}
	return nil
}

func failAuth(w http.ResponseWriter, msg string) {
	log.Printf("FAIL: %s", msg)
	//fmt.Fprintf(w, "FAIL: %s", msg)
	w.WriteHeader(http.StatusBadRequest)
}

type Response struct {
	Message string
	Domain  string
	Headers http.Header
}

type AuthController struct {
	Specs []*EndpointSpec
}

func main() {
	specs := []*EndpointSpec{
		{
			ServiceName: "api-server",
			AuthType:    authTypeDomain,
			Regex:       "/v[23]/domains/([^/]+)",
		},
	}

	if err := compileRegex(specs); err != nil {
		log.Fatalf("Failed to compile spec regexes: %s", err)
	}

	c := AuthController{Specs: specs}
	r := chi.NewRouter()
	r.Get("/", c.getIndex)
	r.Get("/auth", c.getAuth)

	log.Print("listening on 4000....")
	http.ListenAndServe(":4000", r)
}

func (a *AuthController) getIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	b, _ := json.MarshalIndent(Response{Message: "Hello World", Headers: r.Header}, "", " ")
	w.Write(b)
}

func (a *AuthController) getAuth(w http.ResponseWriter, r *http.Request) {
	log.Print("GET /auth")
	header := r.Header.Get("Authorization")
	if header == "" {
		failAuth(w, "missing 'Authorization' header")
		return
	}

	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || parts[0] != "Basic" {
		failAuth(w, "malformed 'Authorization' header; missing 'Basic' prefix")
		return
	}

	payload, _ := base64.StdEncoding.DecodeString(parts[1])
	pair := strings.SplitN(string(payload), ":", 2)

	if len(pair) != 2 {
		failAuth(w, "malformed 'Authorization' header; missing key/value pair after decode")
		return
	}

	// This here only for our POC, either the spec should decide what sort
	// of auth should be done or we default to account level auth with flagman
	if pair[0] != "thrawn" && pair[1] != "password" {
		failAuth(w, "invalid username/password")
		return
	}

	// Match the spec against the request path
	if spec := a.matchSpec(r.URL.Path); spec != nil {
		// TODO: Preform auth using the spec and return valid headers for this spec
		h := w.Header()
		h.Set("X-Mailgun-Domain-Id", "domain-id-01")
		h.Set("X-Mailgun-Account-Id", "account-id-01")
		h.Set("X-Spec-Auth-Type", spec.AuthType)
	} else {
		h := w.Header()
		h.Set("X-Mailgun-Account-Id", "account-id-01")
	}
	w.WriteHeader(http.StatusOK)
	log.Print("GET /auth (AUTH OK)")
}

func (a *AuthController) matchSpec(path string) *EndpointSpec {
	for _, spec := range a.Specs {
		groups := spec.regex.FindStringSubmatch(path)
		if len(groups) == 0 {
			continue
		}
		return spec
	}
	return nil
}

