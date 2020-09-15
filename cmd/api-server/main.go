package main

import (
	"encoding/json"
	"github.com/go-chi/chi"
	"log"
	"net/http"
)

type Response struct {
	Message string
	Domain  string
	Headers http.Header
}

func main() {
	r := chi.NewRouter()

	log.Print("listening on 8081....")
	r.Get("/", getIndex)
	r.Get("/stats", getStats)
	r.Get("/v3/domains/{domain}/info", getDomainInfo)
	http.ListenAndServe(":8081", r)
}

func getIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	b, _ := json.MarshalIndent(Response{Message: "Hello World", Headers: r.Header}, "", " ")
	w.Write(b)
}

func getStats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	b, _ := json.MarshalIndent(Response{Message: "Stats here", Headers: r.Header}, "", " ")
	w.Write(b)
}

func getDomainInfo(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	domain := chi.URLParam(r, "domain")
	if domain == "" {
		http.Error(w, "{domain} in path missing", 422)
	}

	b, _ := json.MarshalIndent(Response{Message: "Domain Handler", Domain: domain, Headers: r.Header}, "", " ")
	w.Write(b)
}
