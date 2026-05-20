package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"rbac-manager/internal/app"
)

func main() {
	addr := getenv("ADDR", ":8080")

	srv := app.NewServer()
	httpServer := &http.Server{
		Addr:              addr,
		Handler:           srv.Routes(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	log.Printf("rbac governance console listening on http://localhost%s", addr)
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
