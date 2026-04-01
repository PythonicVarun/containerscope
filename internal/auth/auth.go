package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"net/http"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const (
	SessionCookieName    = "containerscope_session"
	sessionDuration      = 24 * time.Hour
	shellSessionDuration = 5 * time.Minute // Shell sessions expire quickly for security
	tokenLength          = 32
	bcryptCost           = 10
)

var (
	ErrInvalidCredentials = errors.New("invalid username or password")
	ErrSessionNotFound    = errors.New("session not found")
	ErrSessionExpired     = errors.New("session expired")
	ErrShellTokenInvalid  = errors.New("invalid or expired shell token")
)

type session struct {
	username  string
	createdAt time.Time
	expiresAt time.Time
}

// shellSession represents a temporary session for shell access
type shellSession struct {
	containerID string
	createdAt   time.Time
	expiresAt   time.Time
	used        bool // Can only be used once
}

type Config struct {
	Username     string
	PasswordHash []byte
}

type Manager struct {
	config        Config
	sessions      map[string]*session
	shellSessions map[string]*shellSession
	mu            sync.RWMutex
}

func NewManager(username, password string) (*Manager, error) {
	if username == "" || password == "" {
		return nil, errors.New("username and password are required")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return nil, err
	}

	return &Manager{
		config: Config{
			Username:     username,
			PasswordHash: hash,
		},
		sessions:      make(map[string]*session),
		shellSessions: make(map[string]*shellSession),
	}, nil
}

func (m *Manager) Login(username, password string) (string, error) {
	if subtle.ConstantTimeCompare([]byte(username), []byte(m.config.Username)) != 1 {
		// Still check password to prevent timing attacks
		bcrypt.CompareHashAndPassword(m.config.PasswordHash, []byte(password))
		return "", ErrInvalidCredentials
	}

	if err := bcrypt.CompareHashAndPassword(m.config.PasswordHash, []byte(password)); err != nil {
		return "", ErrInvalidCredentials
	}

	token, err := generateToken()
	if err != nil {
		return "", err
	}

	now := time.Now()
	m.mu.Lock()
	m.sessions[token] = &session{
		username:  username,
		createdAt: now,
		expiresAt: now.Add(sessionDuration),
	}
	m.mu.Unlock()

	return token, nil
}

func (m *Manager) Logout(token string) {
	m.mu.Lock()
	delete(m.sessions, token)
	m.mu.Unlock()
}

func (m *Manager) ValidateSession(token string) error {
	m.mu.RLock()
	sess, exists := m.sessions[token]
	m.mu.RUnlock()

	if !exists {
		return ErrSessionNotFound
	}

	if time.Now().After(sess.expiresAt) {
		m.mu.Lock()
		delete(m.sessions, token)
		m.mu.Unlock()
		return ErrSessionExpired
	}

	return nil
}

func (m *Manager) GetSessionFromRequest(r *http.Request) (string, error) {
	cookie, err := r.Cookie(SessionCookieName)
	if err != nil {
		return "", ErrSessionNotFound
	}

	if err := m.ValidateSession(cookie.Value); err != nil {
		return "", err
	}

	return cookie.Value, nil
}

func (m *Manager) SetSessionCookie(w http.ResponseWriter, token string, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   int(sessionDuration.Seconds()),
	})
}

func (m *Manager) ClearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})
}

func (m *Manager) CleanupExpiredSessions() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	for token, sess := range m.sessions {
		if now.After(sess.expiresAt) {
			delete(m.sessions, token)
		}
	}
}

// VerifyPassword checks if the provided password matches the configured password
func (m *Manager) VerifyPassword(password string) bool {
	err := bcrypt.CompareHashAndPassword(m.config.PasswordHash, []byte(password))
	return err == nil
}

// CreateShellSession creates a new shell session token after password verification
func (m *Manager) CreateShellSession(containerID string, password string) (string, error) {
	// First verify the password
	if !m.VerifyPassword(password) {
		return "", ErrInvalidCredentials
	}

	token, err := generateToken()
	if err != nil {
		return "", err
	}

	now := time.Now()
	m.mu.Lock()
	m.shellSessions[token] = &shellSession{
		containerID: containerID,
		createdAt:   now,
		expiresAt:   now.Add(shellSessionDuration),
		used:        false,
	}
	m.mu.Unlock()

	return token, nil
}

// ValidateAndConsumeShellSession validates a shell token and marks it as used
// Returns the container ID if valid, error otherwise
func (m *Manager) ValidateAndConsumeShellSession(token string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	sess, exists := m.shellSessions[token]
	if !exists {
		return "", ErrShellTokenInvalid
	}

	// Check if expired
	if time.Now().After(sess.expiresAt) {
		delete(m.shellSessions, token)
		return "", ErrShellTokenInvalid
	}

	// Check if already used (single-use tokens)
	if sess.used {
		delete(m.shellSessions, token)
		return "", ErrShellTokenInvalid
	}

	// Mark as used and return container ID
	sess.used = true
	containerID := sess.containerID

	// Delete immediately after use for security
	delete(m.shellSessions, token)

	return containerID, nil
}

// CleanupExpiredShellSessions removes expired shell sessions
func (m *Manager) CleanupExpiredShellSessions() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	for token, sess := range m.shellSessions {
		if now.After(sess.expiresAt) {
			delete(m.shellSessions, token)
		}
	}
}

func generateToken() (string, error) {
	bytes := make([]byte, tokenLength)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}
