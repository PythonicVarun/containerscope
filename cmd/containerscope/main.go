package main

import (
	"errors"
	"log"
	"net/http"
	"os"
	"strings"

	"containerscope/internal/app"
)

const (
	defaultPort     = "4000"
	defaultUsername = "admin"
)

func main() {
	port := strings.TrimSpace(os.Getenv("CONTAINER_SCOPE_PORT"))
	if port == "" {
		port = defaultPort
	}

	username := strings.TrimSpace(os.Getenv("CONTAINER_SCOPE_USERNAME"))
	if username == "" {
		username = defaultUsername
	}

	password := strings.TrimSpace(os.Getenv("CONTAINER_SCOPE_PASSWORD"))
	if password == "" {
		log.Fatal("CONTAINER_SCOPE_PASSWORD environment variable is required")
	}

	secureCookies := strings.ToLower(strings.TrimSpace(os.Getenv("CONTAINER_SCOPE_SECURE_COOKIES"))) == "true"

	server, err := app.NewServer(app.Config{
		Port:          port,
		PublicDir:     "public",
		AuthUsername:  username,
		AuthPassword:  password,
		SecureCookies: secureCookies,
	})
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	log.Printf("ContainerScope server running on http://0.0.0.0:%s", port)
	log.Printf("Login with username: %s", username)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}
