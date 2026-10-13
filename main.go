package main

import (
	"context"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var lookups = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "dns_lookups_total",
	Help: "DNS lookups by host and status.",
}, []string{"host", "status"})

func resolve(ctx context.Context, host string) {
	retryCount := getEnv("RETRY_COUNT", "3")
	retryDelay := getEnv("RETRY_DELAY_MS", "100")
	delay, err := strconv.Atoi(retryDelay)
	if err != nil || delay < 0 {
		delay = 100
	}
	count, err := strconv.Atoi(retryCount)
	if err != nil || count <= 0 {
		count = 3
	}
	status := "success"
	for i := 0; i < count; i++ {
		if _, err := net.DefaultResolver.LookupHost(ctx, host); err != nil {
			log.Print("Failed to resolve ", host, ": ", err, " (attempt ", i+1, " of ", retryCount, ")")
			time.Sleep(time.Duration(delay) * time.Millisecond)
			if i == count-1 {
				status = "failure"
			}
		}
	}

	lookups.WithLabelValues(host, status).Inc()
}

func main() {
	var targets []string
	for _, t := range strings.Split(getEnv("DOMAINS", "google.com,testbogus.com"), ",") {
		if t = strings.TrimSpace(t); t != "" {
			targets = append(targets, t)
			// Pre-initialize so both series show up as 0 before the first lookup.
			lookups.WithLabelValues(t, "success")
			lookups.WithLabelValues(t, "failure")
		}
	}
	port := getEnv("PORT", "8081")

	http.Handle("/metrics", promhttp.Handler())
	http.Handle("/", promhttp.Handler())

	go func() {
		lookupRate := getEnv("LOOKUP_RATE", "4")
		rate, err := strconv.Atoi(lookupRate)
		if err != nil || rate <= 0 {
			rate = 4
		}
		ticker := time.NewTicker(time.Second / time.Duration(rate))

		defer ticker.Stop()
		for range ticker.C {
			for _, target := range targets {
				go func(host string) {
					ctx, cancel := context.WithTimeout(context.Background(), time.Second)
					defer cancel()
					resolve(ctx, host)
				}(target)
			}
		}
	}()

	log.Printf("Listening on :%s/metrics, testing %v every second", port, targets)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}
