package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/go-chi/chi"
	poc "github.com/thrawn01/poc"
)

type Response struct {
	Message string
	Domain  string
	Headers http.Header
}

var (
	limiter *poc.Limiter
)

func main() {
	mb, err := strconv.ParseInt(os.Getenv("MB_LIMIT"), 10, 64)
	if err != nil {
		panic(err)
	}
	limiter = poc.NewConnLimiter(mb)
	r := chi.NewRouter()

	log.Printf("[%s] listening on 80....", os.Getenv("NAME"))

	// For testing auth with traefik
	r.Get("/", getIndex)
	r.Get("/stats", getStats)
	r.Get("/v3/domains/{domain}/info", getDomainInfo)

	// For testing load balancer off loading with limiter
	r.Get("/health", checkHealth)
	r.Head("/health", checkHealth)
	r.Get("/v3/messages", sendMessages)

	// Let's GO!
	err = http.ListenAndServe(":80", r)
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		panic(err)
	}
}

func getIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	b, _ := json.MarshalIndent(Response{Message: "Hello World", Headers: r.Header}, "", " ")
	_, _ = w.Write(b)
}

func getStats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	b, _ := json.MarshalIndent(Response{Message: "Stats here", Headers: r.Header}, "", " ")
	_, _ = w.Write(b)
}

func getDomainInfo(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	domain := chi.URLParam(r, "domain")
	if domain == "" {
		http.Error(w, "{domain} in path missing", 422)
	}

	b, _ := json.MarshalIndent(Response{Message: "Domain Handler", Domain: domain, Headers: r.Header}, "", " ")
	_, _ = w.Write(b)
}

func sendMessages(w http.ResponseWriter, r *http.Request) {
	// Between 0 and 5MB
	size := rand.Int63n(5 << 20)

	// Tell the limiter about the random data we received
	defer func() { limiter.Decrement(size) }()
	limiter.Increment(size)

	// Sleep between 100ms and 10 seconds
	time.Sleep(time.Duration(100+rand.Intn(10901)) * time.Millisecond)

	// TODO: Maybe we should have a soft limit, and a HARD limit?
	// 	Soft limit to inform Traefik to back off
	//  Hard limit to protect influx from OOM

	// Always return okay, regardless of the limiter
	_, _ = fmt.Fprintf(w, "[%s] Queued (%d), OK", os.Getenv("NAME"), size)
}

var last503 = time.Now()

func checkHealth(w http.ResponseWriter, r *http.Request) {

	// If we are under our requested limit, return a healthy response.
	if limiter.IsOverLimit() {
		w.WriteHeader(http.StatusServiceUnavailable)
		fmt.Printf("[%s] Service is Overloaded at %d Bytes %.2f MB\n", os.Getenv("NAME"), limiter.Current(),
			float64(limiter.Current())/float64(1<<20))
		_, _ = fmt.Fprintf(w, "[%s] Service is Overloaded", os.Getenv("NAME"))
		last503 = time.Now().Add(time.Second * 15)
		return
	}

	// For testing only, once we trigger 503, then we should cool down before allowing traffic again.
	// This allows us to visually see the load balancer only allow traffic to the other api-server.
	if time.Now().Before(last503) {
		w.WriteHeader(http.StatusServiceUnavailable)
		fmt.Printf("[%s] Service is Overloaded COOLDOWN at %d Bytes %.2f MB\n", os.Getenv("NAME"), limiter.Current(),
			float64(limiter.Current())/float64(1<<20))
		_, _ = fmt.Fprintf(w, "[%s] Service is Overloaded COOLDOWN", os.Getenv("NAME"))
		return
	}

	fmt.Printf("[%s] Service is Healthy at %d Bytes %.2f MB\n", os.Getenv("NAME"), limiter.Current(),
		float64(limiter.Current())/float64(1<<20))
	_, _ = fmt.Fprintf(w, "[%s] Service is Healthy", os.Getenv("NAME"))
}
