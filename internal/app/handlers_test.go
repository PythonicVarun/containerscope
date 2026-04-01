package app

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"containerscope/internal/auth"
	"containerscope/internal/dockerapi"
)

func TestNewHandlerLoginCheckAndLogoutFlow(t *testing.T) {
	handler, _, _ := newTestHandler(t)

	loginReq := httptest.NewRequest(http.MethodPost, "/api/login", bytes.NewBufferString(`{"username":"admin","password":"secret"}`))
	loginReq.Header.Set("Content-Type", "application/json")
	loginReq.Header.Set("X-Real-IP", "203.0.113.9")

	loginRec := httptest.NewRecorder()
	handler.ServeHTTP(loginRec, loginReq)

	if loginRec.Code != http.StatusOK {
		t.Fatalf("expected login status %d, got %d", http.StatusOK, loginRec.Code)
	}

	var loginResp map[string]string
	if err := json.NewDecoder(loginRec.Body).Decode(&loginResp); err != nil {
		t.Fatalf("failed to decode login response: %v", err)
	}
	if loginResp["status"] != "ok" {
		t.Fatalf("expected login status ok, got %#v", loginResp)
	}

	sessionCookie := findCookie(loginRec.Result().Cookies(), auth.SessionCookieName)
	if sessionCookie == nil {
		t.Fatal("expected login to set the session cookie")
	}
	if !sessionCookie.HttpOnly {
		t.Fatal("expected session cookie to be HttpOnly")
	}
	if !sessionCookie.Secure {
		t.Fatal("expected session cookie to be Secure")
	}

	checkReq := httptest.NewRequest(http.MethodGet, "/api/auth/check", nil)
	checkReq.AddCookie(sessionCookie)

	checkRec := httptest.NewRecorder()
	handler.ServeHTTP(checkRec, checkReq)

	if checkRec.Code != http.StatusOK {
		t.Fatalf("expected auth check status %d, got %d", http.StatusOK, checkRec.Code)
	}

	var checkResp map[string]string
	if err := json.NewDecoder(checkRec.Body).Decode(&checkResp); err != nil {
		t.Fatalf("failed to decode auth check response: %v", err)
	}
	if checkResp["authenticated"] != "true" {
		t.Fatalf("expected authenticated=true, got %#v", checkResp)
	}

	logoutReq := httptest.NewRequest(http.MethodPost, "/api/logout", nil)
	logoutReq.AddCookie(sessionCookie)

	logoutRec := httptest.NewRecorder()
	handler.ServeHTTP(logoutRec, logoutReq)

	if logoutRec.Code != http.StatusOK {
		t.Fatalf("expected logout status %d, got %d", http.StatusOK, logoutRec.Code)
	}

	clearedCookie := findCookie(logoutRec.Result().Cookies(), auth.SessionCookieName)
	if clearedCookie == nil {
		t.Fatal("expected logout to clear the session cookie")
	}
	if clearedCookie.MaxAge >= 0 {
		t.Fatalf("expected cleared cookie MaxAge < 0, got %d", clearedCookie.MaxAge)
	}

	checkAfterLogoutReq := httptest.NewRequest(http.MethodGet, "/api/auth/check", nil)
	checkAfterLogoutReq.AddCookie(sessionCookie)

	checkAfterLogoutRec := httptest.NewRecorder()
	handler.ServeHTTP(checkAfterLogoutRec, checkAfterLogoutReq)

	if checkAfterLogoutRec.Code != http.StatusUnauthorized {
		t.Fatalf("expected auth check after logout status %d, got %d", http.StatusUnauthorized, checkAfterLogoutRec.Code)
	}
}

func TestNewHandlerRedirectsAndServesPublicFiles(t *testing.T) {
	handler, manager, _ := newTestHandler(t)

	redirectReq := httptest.NewRequest(http.MethodGet, "/containers?filter=running", nil)
	redirectRec := httptest.NewRecorder()
	handler.ServeHTTP(redirectRec, redirectReq)

	if redirectRec.Code != http.StatusFound {
		t.Fatalf("expected redirect status %d, got %d", http.StatusFound, redirectRec.Code)
	}
	if location := redirectRec.Header().Get("Location"); location != "/login?next=%2Fcontainers%3Ffilter%3Drunning" {
		t.Fatalf("unexpected redirect location: %q", location)
	}

	loginPageReq := httptest.NewRequest(http.MethodGet, "/login", nil)
	loginPageRec := httptest.NewRecorder()
	handler.ServeHTTP(loginPageRec, loginPageReq)

	if loginPageRec.Code != http.StatusOK {
		t.Fatalf("expected login page status %d, got %d", http.StatusOK, loginPageRec.Code)
	}
	if !strings.Contains(loginPageRec.Body.String(), "login page") {
		t.Fatalf("expected login page body, got %q", loginPageRec.Body.String())
	}

	staticReq := httptest.NewRequest(http.MethodGet, "/app.js", nil)
	staticRec := httptest.NewRecorder()
	handler.ServeHTTP(staticRec, staticReq)

	if staticRec.Code != http.StatusOK {
		t.Fatalf("expected static asset status %d, got %d", http.StatusOK, staticRec.Code)
	}
	if !strings.Contains(staticRec.Body.String(), "console.log") {
		t.Fatalf("expected JavaScript asset body, got %q", staticRec.Body.String())
	}

	token, err := manager.Login("admin", "secret")
	if err != nil {
		t.Fatalf("Login returned error: %v", err)
	}

	indexReq := httptest.NewRequest(http.MethodGet, "/", nil)
	indexReq.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token})

	indexRec := httptest.NewRecorder()
	handler.ServeHTTP(indexRec, indexReq)

	if indexRec.Code != http.StatusOK {
		t.Fatalf("expected index status %d, got %d", http.StatusOK, indexRec.Code)
	}
	if !strings.Contains(indexRec.Body.String(), "home page") {
		t.Fatalf("expected index page body, got %q", indexRec.Body.String())
	}
}

func TestNewHandlerShellSessionEndpointRequiresAuthAndValidPassword(t *testing.T) {
	handler, manager, _ := newTestHandler(t)

	unauthorizedReq := httptest.NewRequest(http.MethodPost, "/api/containers/container-123/shell", bytes.NewBufferString(`{"password":"secret"}`))
	unauthorizedReq.Header.Set("Content-Type", "application/json")

	unauthorizedRec := httptest.NewRecorder()
	handler.ServeHTTP(unauthorizedRec, unauthorizedReq)

	if unauthorizedRec.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthorized status %d, got %d", http.StatusUnauthorized, unauthorizedRec.Code)
	}

	token, err := manager.Login("admin", "secret")
	if err != nil {
		t.Fatalf("Login returned error: %v", err)
	}
	sessionCookie := &http.Cookie{Name: auth.SessionCookieName, Value: token}

	invalidPasswordReq := httptest.NewRequest(http.MethodPost, "/api/containers/container-123/shell", bytes.NewBufferString(`{"password":"wrong-password"}`))
	invalidPasswordReq.Header.Set("Content-Type", "application/json")
	invalidPasswordReq.AddCookie(sessionCookie)

	invalidPasswordRec := httptest.NewRecorder()
	handler.ServeHTTP(invalidPasswordRec, invalidPasswordReq)

	if invalidPasswordRec.Code != http.StatusUnauthorized {
		t.Fatalf("expected invalid password status %d, got %d", http.StatusUnauthorized, invalidPasswordRec.Code)
	}

	validReq := httptest.NewRequest(http.MethodPost, "/api/containers/container-123/shell", bytes.NewBufferString(`{"password":"secret"}`))
	validReq.Header.Set("Content-Type", "application/json")
	validReq.AddCookie(sessionCookie)

	validRec := httptest.NewRecorder()
	handler.ServeHTTP(validRec, validReq)

	if validRec.Code != http.StatusOK {
		t.Fatalf("expected shell session status %d, got %d", http.StatusOK, validRec.Code)
	}

	var shellResp map[string]string
	if err := json.NewDecoder(validRec.Body).Decode(&shellResp); err != nil {
		t.Fatalf("failed to decode shell session response: %v", err)
	}
	if shellResp["status"] != "ok" {
		t.Fatalf("expected shell session status ok, got %#v", shellResp)
	}
	if shellResp["containerId"] != "container-123" {
		t.Fatalf("expected containerId container-123, got %#v", shellResp)
	}
	if shellResp["shellToken"] == "" {
		t.Fatal("expected shellToken to be returned")
	}

	containerID, err := manager.ValidateAndConsumeShellSession(shellResp["shellToken"])
	if err != nil {
		t.Fatalf("expected shell token to be usable, got error %v", err)
	}
	if containerID != "container-123" {
		t.Fatalf("expected shell token for container-123, got %q", containerID)
	}
}

func newTestHandler(t *testing.T) (http.Handler, *auth.Manager, *auth.RateLimiter) {
	t.Helper()

	publicDir := t.TempDir()
	writeTestFile(t, filepath.Join(publicDir, "login.html"), "<html><body>login page</body></html>")
	writeTestFile(t, filepath.Join(publicDir, "index.html"), "<html><body>home page</body></html>")
	writeTestFile(t, filepath.Join(publicDir, "app.js"), "console.log('test asset');")

	manager, err := auth.NewManager("admin", "secret")
	if err != nil {
		t.Fatalf("NewManager returned error: %v", err)
	}

	limiter := auth.NewRateLimiter()
	handler := NewHandler(publicDir, &dockerapi.Client{}, manager, limiter, true, dockerapi.DefaultSocketPath)
	return handler, manager, limiter
}

func writeTestFile(t *testing.T, path string, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("failed to write %s: %v", path, err)
	}
}

func findCookie(cookies []*http.Cookie, name string) *http.Cookie {
	for _, cookie := range cookies {
		if cookie.Name == name {
			return cookie
		}
	}
	return nil
}
