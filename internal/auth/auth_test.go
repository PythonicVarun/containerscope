package auth

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestManagerLoginAndRequestSession(t *testing.T) {
	manager, err := NewManager("admin", "secret")
	if err != nil {
		t.Fatalf("NewManager returned error: %v", err)
	}

	if _, err := manager.Login("admin", "wrong-password"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected invalid credentials error, got %v", err)
	}

	token, err := manager.Login("admin", "secret")
	if err != nil {
		t.Fatalf("Login returned error: %v", err)
	}

	if err := manager.ValidateSession(token); err != nil {
		t.Fatalf("ValidateSession returned error: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: token})

	gotToken, err := manager.GetSessionFromRequest(req)
	if err != nil {
		t.Fatalf("GetSessionFromRequest returned error: %v", err)
	}
	if gotToken != token {
		t.Fatalf("expected token %q, got %q", token, gotToken)
	}

	manager.Logout(token)
	if err := manager.ValidateSession(token); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("expected session not found after logout, got %v", err)
	}
}

func TestManagerValidateSessionExpiresAndRemovesToken(t *testing.T) {
	manager, err := NewManager("admin", "secret")
	if err != nil {
		t.Fatalf("NewManager returned error: %v", err)
	}

	manager.sessions["expired-token"] = &session{
		username:  "admin",
		createdAt: time.Now().Add(-2 * time.Hour),
		expiresAt: time.Now().Add(-time.Minute),
	}

	if err := manager.ValidateSession("expired-token"); !errors.Is(err, ErrSessionExpired) {
		t.Fatalf("expected expired session error, got %v", err)
	}

	if _, exists := manager.sessions["expired-token"]; exists {
		t.Fatal("expected expired token to be deleted")
	}
}

func TestManagerShellSessionRequiresPasswordAndIsSingleUse(t *testing.T) {
	manager, err := NewManager("admin", "secret")
	if err != nil {
		t.Fatalf("NewManager returned error: %v", err)
	}

	if _, err := manager.CreateShellSession("container-123", "wrong-password"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected invalid credentials error, got %v", err)
	}

	token, err := manager.CreateShellSession("container-123", "secret")
	if err != nil {
		t.Fatalf("CreateShellSession returned error: %v", err)
	}

	containerID, err := manager.ValidateAndConsumeShellSession(token)
	if err != nil {
		t.Fatalf("ValidateAndConsumeShellSession returned error: %v", err)
	}
	if containerID != "container-123" {
		t.Fatalf("expected container ID %q, got %q", "container-123", containerID)
	}

	if _, err := manager.ValidateAndConsumeShellSession(token); !errors.Is(err, ErrShellTokenInvalid) {
		t.Fatalf("expected invalid shell token on second use, got %v", err)
	}

	manager.shellSessions["expired-shell-token"] = &shellSession{
		containerID: "container-456",
		createdAt:   time.Now().Add(-10 * time.Minute),
		expiresAt:   time.Now().Add(-time.Minute),
	}

	if _, err := manager.ValidateAndConsumeShellSession("expired-shell-token"); !errors.Is(err, ErrShellTokenInvalid) {
		t.Fatalf("expected expired shell token error, got %v", err)
	}

	if _, exists := manager.shellSessions["expired-shell-token"]; exists {
		t.Fatal("expected expired shell token to be deleted")
	}
}
