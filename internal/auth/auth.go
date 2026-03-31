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
	SessionCookieName = "containerscope_session"
	sessionDuration   = 24 * time.Hour
	tokenLength       = 32
	bcryptCost        = 10
)

var (
	ErrInvalidCredentials = errors.New("invalid username or password")
	ErrSessionNotFound    = errors.New("session not found")
	ErrSessionExpired     = errors.New("session expired")
)

type session struct {
	username  string
	createdAt time.Time
	expiresAt time.Time
}

type Config struct {
	Username     string
	PasswordHash []byte
}

type Manager struct {
	config   Config
	sessions map[string]*session
	mu       sync.RWMutex
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
		sessions: make(map[string]*session),
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

func generateToken() (string, error) {
	bytes := make([]byte, tokenLength)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}
