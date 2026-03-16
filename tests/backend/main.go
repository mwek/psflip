package main

import (
	"context"
	"fmt"
	"math/rand/v2"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

func main() {
	// Random crash during startup based on CRASH_RATE (0-100)
	if crashRate := getCrashRate(); crashRate > 0 {
		if rand.IntN(100) < crashRate {
			fmt.Fprintln(os.Stderr, "simulated startup crash")
			os.Exit(1)
		}
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	delay := os.Getenv("DELAY")
	if delay == "" {
		delay = "50ms"
	}
	delayDuration, err := time.ParseDuration(delay)
	if err != nil {
		fmt.Fprintln(os.Stderr, "invalid delay:", err)
		os.Exit(1)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(delayDuration)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Fprintln(os.Stderr, "server error:", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "shutdown error:", err)
		os.Exit(1)
	}
}

func getCrashRate() int {
	s := os.Getenv("CRASH_RATE")
	if s == "" {
		return 0
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 || n > 100 {
		return 0
	}
	return n
}
