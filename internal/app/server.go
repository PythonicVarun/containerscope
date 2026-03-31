package app

import (
	"net/http"
	"time"

	"containerscope/internal/dockerapi"
)

type Config struct {
	Port             string
	PublicDir        string
	DockerSocketPath string
}

func NewServer(cfg Config) *http.Server {
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

	return &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           NewHandler(cfg.PublicDir, docker),
		ReadHeaderTimeout: 5 * time.Second,
	}
}
