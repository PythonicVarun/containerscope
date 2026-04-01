package app

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"

	"containerscope/internal/auth"
	"containerscope/internal/dockerapi"
	"containerscope/internal/ws"
)

type logsResponse struct {
	Logs []dockerapi.LogLine `json:"logs"`
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type shellRequest struct {
	Password string `json:"password"`
}

func NewHandler(publicDir string, docker *dockerapi.Client, authManager *auth.Manager, rateLimiter *auth.RateLimiter, secureCookies bool, socketPath string) http.Handler {
	fileServer := http.FileServer(http.Dir(publicDir))
	mux := http.NewServeMux()

	// Auth endpoints (public)
	mux.HandleFunc("/api/login", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		clientIP := auth.GetClientIP(r)

		// Check rate limiting
		if locked, remaining := rateLimiter.IsLocked(clientIP); locked {
			writeJSON(w, http.StatusTooManyRequests, map[string]any{
				"error":       "too many failed login attempts",
				"retry_after": int(remaining.Seconds()),
			})
			return
		}

		var req loginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		token, err := authManager.Login(req.Username, req.Password)
		if err != nil {
			rateLimiter.RecordFailure(clientIP)
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
			return
		}

		rateLimiter.RecordSuccess(clientIP)
		authManager.SetSessionCookie(w, token, secureCookies)
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	mux.HandleFunc("/api/logout", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		if cookie, err := r.Cookie(auth.SessionCookieName); err == nil {
			authManager.Logout(cookie.Value)
		}

		authManager.ClearSessionCookie(w)
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	mux.HandleFunc("/api/auth/check", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		if _, err := authManager.GetSessionFromRequest(r); err != nil {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"authenticated": "false"})
			return
		}

		writeJSON(w, http.StatusOK, map[string]string{"authenticated": "true"})
	})

	// Protected API endpoints
	mux.HandleFunc("/api/containers", authManager.RequireAuth(func(w http.ResponseWriter, r *http.Request) {
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
	}))

	mux.HandleFunc("/api/logs/", authManager.RequireAuth(func(w http.ResponseWriter, r *http.Request) {
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
	}))

	// WebSocket endpoint (protected)
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		// Validate session before upgrading WebSocket connection
		if _, err := authManager.GetSessionFromRequest(r); err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		conn, err := ws.Upgrade(w, r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		ws.NewSession(conn, docker).Run()
	})

	// Shell WebSocket endpoint (protected, requires password re-verification)
	mux.HandleFunc("/api/containers/", authManager.RequireAuth(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api/containers/")
		parts := strings.SplitN(path, "/", 2)

		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			http.NotFound(w, r)
			return
		}

		id, action := parts[0], parts[1]

		// Handle shell action separately
		if action == "shell" {
			if r.Method != http.MethodPost {
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}

			var req shellRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, "invalid request body", http.StatusBadRequest)
				return
			}

			// Create shell session (verifies password internally)
			shellToken, err := authManager.CreateShellSession(id, req.Password)
			if err != nil {
				writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid password"})
				return
			}

			// Return the shell token for WebSocket connection
			writeJSON(w, http.StatusOK, map[string]string{
				"status":      "ok",
				"containerId": id,
				"shellToken":  shellToken,
			})
			return
		}

		// Handle other container actions
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

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
	}))

	// Shell WebSocket endpoint
	mux.HandleFunc("/ws/shell/", func(w http.ResponseWriter, r *http.Request) {
		// Validate main session first
		if _, err := authManager.GetSessionFromRequest(r); err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		// Extract shell token from query parameter
		shellToken := r.URL.Query().Get("token")
		if shellToken == "" {
			http.Error(w, "shell token required", http.StatusBadRequest)
			return
		}

		// Validate and consume shell token (single-use)
		containerID, err := authManager.ValidateAndConsumeShellSession(shellToken)
		if err != nil {
			http.Error(w, "invalid or expired shell token", http.StatusUnauthorized)
			return
		}

		// Verify the container ID in the URL matches the token's container ID
		urlContainerID := strings.TrimPrefix(r.URL.Path, "/ws/shell/")
		if urlContainerID != containerID {
			http.Error(w, "container ID mismatch", http.StatusForbidden)
			return
		}

		conn, err := ws.Upgrade(w, r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		ws.NewShellSession(conn, docker, socketPath).Run(containerID)
	})

	// Static files and login page
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Serve login page without auth
		if r.URL.Path == "/login" || r.URL.Path == "/login.html" {
			http.ServeFile(w, r, filepath.Join(publicDir, "login.html"))
			return
		}

		// Static assets (CSS, JS, images) are public
		if isStaticAsset(r.URL.Path) {
			fileServer.ServeHTTP(w, r)
			return
		}

		// Protected: main app requires authentication
		if _, err := authManager.GetSessionFromRequest(r); err != nil {
			// Redirect to login page
			nextURL := r.URL.Path
			if r.URL.RawQuery != "" {
				nextURL += "?" + r.URL.RawQuery
			}

			redirectURL := "/login?next=" + url.QueryEscape(nextURL)
			http.Redirect(w, r, redirectURL, http.StatusFound)
			return
		}

		if r.URL.Path == "/" {
			http.ServeFile(w, r, filepath.Join(publicDir, "index.html"))
			return
		}
		fileServer.ServeHTTP(w, r)
	})

	return mux
}

func isStaticAsset(path string) bool {
	staticExtensions := []string{".css", ".js", ".svg", ".png", ".ico", ".woff", ".woff2", ".ttf"}
	for _, ext := range staticExtensions {
		if strings.HasSuffix(path, ext) {
			return true
		}
	}
	return false
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
