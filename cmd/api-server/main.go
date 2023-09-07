package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/go-chi/chi"
	"github.com/prometheus/client_golang/prometheus"
	poc "github.com/thrawn01/poc"
)

type Response struct {
	Message string
	Domain  string
	Headers http.Header
}

var (
	limiter        *poc.Limiter
	slow           time.Duration
	mb             int64
	metricRequests = prometheus.NewSummary(prometheus.SummaryOpts{
		Name: "http_requests",
		Help: "The duration of http requests",
		Objectives: map[float64]float64{
			0.5:  0.05,
			0.95: 0.01,
			0.99: 0.001,
			1:    0.001,
		},
	})
)

func main() {
	var err error

	promRegister := prometheus.NewRegistry()
	promRegister.MustRegister(metricRequests)

	mb, err = strconv.ParseInt(os.Getenv("MB_LIMIT"), 10, 64)
	if err != nil {
		panic(err)
	}
	slow, err = time.ParseDuration(os.Getenv("SLOW"))
	if err != nil {
		slow = time.Nanosecond
	}
	fmt.Printf("Slow: %s MB: %d\n", slow.String(), mb)

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

	r.Handle("/metrics", promhttp.InstrumentMetricHandler(
		promRegister, promhttp.HandlerFor(promRegister, promhttp.HandlerOpts{}),
	))

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
	defer prometheus.NewTimer(metricRequests).ObserveDuration()

	// Between 0 and 5MB
	size := rand.Int63n(5 << 20)

	// Tell the limiter about the random data we received
	defer func() { limiter.Decrement(size) }()
	limiter.Increment(size)

	// Sleep between 100ms and 10 seconds
	if mb != 0 {
		time.Sleep(time.Duration(100+rand.Intn(10901)) * time.Millisecond)
	}

	// Add some artificial slowness in addition to the above-simulated time
	// it takes to send data over the wire
	time.Sleep(slow)

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
