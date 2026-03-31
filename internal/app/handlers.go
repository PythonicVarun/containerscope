package app

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"strings"

	"containerscope/internal/dockerapi"
	"containerscope/internal/ws"
)

type logsResponse struct {
	Logs []dockerapi.LogLine `json:"logs"`
}

func NewHandler(publicDir string, docker *dockerapi.Client) http.Handler {
	fileServer := http.FileServer(http.Dir(publicDir))
	mux := http.NewServeMux()

	mux.HandleFunc("/api/containers", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		containers, err := docker.ListContainers(r.Context())
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		writeJSON(w, http.StatusOK, containers)
	})

	mux.HandleFunc("/api/containers/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		path := strings.TrimPrefix(r.URL.Path, "/api/containers/")
		parts := strings.SplitN(path, "/", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			http.NotFound(w, r)
			return
		}

		id, action := parts[0], parts[1]
		var err error

		switch action {
		case "start":
			err = docker.StartContainer(r.Context(), id)
		case "stop":
			err = docker.StopContainer(r.Context(), id)
		case "restart":
			err = docker.RestartContainer(r.Context(), id)
		default:
			http.NotFound(w, r)
			return
		}

		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	mux.HandleFunc("/api/logs/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		id, ok := resourceID(r.URL.Path, "/api/logs/")
		if !ok {
			http.NotFound(w, r)
			return
		}

		stream, err := docker.OpenLogs(r.Context(), id, 200, false)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		defer stream.Body.Close()

		data, err := io.ReadAll(stream.Body)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		writeJSON(w, http.StatusOK, logsResponse{
			Logs: dockerapi.ParseHistory(data, stream.TTY),
		})
	})

	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		conn, err := ws.Upgrade(w, r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		ws.NewSession(conn, docker).Run()
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.ServeFile(w, r, filepath.Join(publicDir, "index.html"))
			return
		}
		fileServer.ServeHTTP(w, r)
	})

	return mux
}

func resourceID(path, prefix string) (string, bool) {
	id := strings.TrimSpace(strings.TrimPrefix(path, prefix))
	if id == "" || strings.Contains(id, "/") {
		return "", false
	}
	return id, true
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Printf("json encode error: %v", err)
	}
}
