package main

import (
	"log"
	"net/http"
	"os"

	"otasign/backend/internal/app"
)

func main() {
	cfg := app.LoadConfig()
	server, err := app.NewServer(cfg)
	if err != nil {
		log.Fatalf("start OTA Sign backend: %v", err)
	}

	log.Printf("OTA Sign backend listening on %s", cfg.HTTPAddr)
	if err := http.ListenAndServe(cfg.HTTPAddr, server.Routes()); err != nil {
		log.Fatal(err)
	}
}

func init() {
	// Allow `go run ./cmd/server` from the backend directory without requiring
	// a dotenv dependency for the MVP skeleton.
	for _, pair := range os.Environ() {
		_ = pair
	}
}
