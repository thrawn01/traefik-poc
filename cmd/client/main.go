package main

import (
	"context"
	"fmt"
	"golang.org/x/time/rate"
	"io"
	"log"
	"net/http"
	"strings"
)

func request(client *http.Client, limiter *rate.Limiter) {

	ctx := context.Background()
	if err := limiter.Wait(ctx); err != nil {
		log.Printf("Wait error: %v", err)
		return
	}

	req, err := http.NewRequest("GET", "http://localhost:8085/v3/messages", nil)
	if err != nil {
		log.Printf("Error creating request: %v", err)
		return
	}
	req.Host = "api-server.com"

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Error making request: %v", err)
		return
	}

	// Read the response body. This is just to show you can, you might not want to do this for all responses.
	body, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		log.Printf("Error reading response: %v", err)
		return
	}

	fmt.Printf("-> %s\n", strings.Trim(string(body), "\n"))
}

func main() {
	// 5 requests per second with a burst of 1.
	limiter := rate.NewLimiter(1_00, 1)
	client := &http.Client{}

	con := make(chan struct{}, 500)
	for {
		con <- struct{}{}
		go func() {
			request(client, limiter)
			<-con
		}()
	}
}
