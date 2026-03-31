package app

import (
	"net/http"
	"time"

	"containerscope/internal/auth"
	"containerscope/internal/dockerapi"
)

type Config struct {
	Port             string
	PublicDir        string
	DockerSocketPath string
	AuthUsername     string
	AuthPassword     string
	SecureCookies    bool
}

func NewServer(cfg Config) (*http.Server, error) {
	if cfg.Port == "" {
		cfg.Port = "4000"
	}
	if cfg.PublicDir == "" {
		cfg.PublicDir = "public"
	}
	if cfg.DockerSocketPath == "" {
		cfg.DockerSocketPath = dockerapi.DefaultSocketPath
	}

	docker := dockerapi.NewClient(cfg.DockerSocketPath)

	authManager, err := auth.NewManager(cfg.AuthUsername, cfg.AuthPassword)
	if err != nil {
		return nil, err
	}

	rateLimiter := auth.NewRateLimiter()

	return &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           NewHandler(cfg.PublicDir, docker, authManager, rateLimiter, cfg.SecureCookies),
		ReadHeaderTimeout: 5 * time.Second,
	}, nil
}
