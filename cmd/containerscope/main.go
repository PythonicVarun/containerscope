package main

import (
	"errors"
	"log"
	"net/http"
	"os"
	"strings"

	"containerscope/internal/app"
)

const defaultPort = "4000"

func main() {
	port := strings.TrimSpace(os.Getenv("CONTAINER_SCOPE_PORT"))
	if port == "" {
		port = defaultPort
	}

	server := app.NewServer(app.Config{
		Port:      port,
		PublicDir: "public",
	})

	log.Printf("ContainerScope server running on http://0.0.0.0:%s", port)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}
