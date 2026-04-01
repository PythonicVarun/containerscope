package ws

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net"
	"sync"

	"containerscope/internal/dockerapi"
)

type shellExecutor interface {
	CreateExec(ctx context.Context, containerID string) (string, error)
	StartExec(ctx context.Context, execID string, socketPath string) (*dockerapi.ExecSession, error)
	ResizeExec(ctx context.Context, execID string, width, height int) error
}

type ShellSession struct {
	conn       *Conn
	docker     shellExecutor
	socketPath string

	mu     sync.Mutex
	exec   *dockerapi.ExecSession
	execID string
	closed bool
}

type shellMessage struct {
	Type   string `json:"type,omitempty"`
	Data   string `json:"data,omitempty"`
	Width  int    `json:"width,omitempty"`
	Height int    `json:"height,omitempty"`
}

func NewShellSession(conn *Conn, docker shellExecutor, socketPath string) *ShellSession {
	return &ShellSession{
		conn:       conn,
		docker:     docker,
		socketPath: socketPath,
	}
}

func (s *ShellSession) Run(containerID string) {
	defer s.cleanup()

	ctx := context.Background()

	// Create exec instance
	execID, err := s.docker.CreateExec(ctx, containerID)
	if err != nil {
		s.sendError("Failed to create shell: " + err.Error())
		return
	}

	// Start exec and get raw connection
	exec, err := s.docker.StartExec(ctx, execID, s.socketPath)
	if err != nil {
		s.sendError("Failed to start shell: " + err.Error())
		return
	}

	s.mu.Lock()
	s.exec = exec
	s.execID = execID
	s.mu.Unlock()

	// Set initial terminal size (common default)
	_ = s.docker.ResizeExec(ctx, execID, 120, 40)

	// Start bidirectional communication
	done := make(chan struct{})

	// Read from container and send to WebSocket
	go func() {
		defer close(done)
		buf := make([]byte, 4096)
		for {
			n, err := exec.Read(buf)
			if err != nil {
				if !errors.Is(err, io.EOF) && !errors.Is(err, net.ErrClosed) {
					log.Printf("shell read error: %v", err)
				}
				return
			}
			if n > 0 {
				if err := s.conn.WriteJSON(map[string]string{
					"type": "output",
					"data": string(buf[:n]),
				}); err != nil {
					return
				}
			}
		}
	}()

	// Read from WebSocket and send to container
	go func() {
		for {
			msg, err := s.conn.ReadText()
			if err != nil {
				s.mu.Lock()
				if s.exec != nil {
					s.exec.Close()
				}
				s.mu.Unlock()
				return
			}

			// Try to parse as JSON for resize commands
			var shellMsg shellMessage
			if err := json.Unmarshal([]byte(msg), &shellMsg); err == nil && shellMsg.Type == "resize" {
				if shellMsg.Width > 0 && shellMsg.Height > 0 {
					s.mu.Lock()
					if s.execID != "" {
						_ = s.docker.ResizeExec(ctx, s.execID, shellMsg.Width, shellMsg.Height)
					}
					s.mu.Unlock()
				}
				continue
			}

			// Otherwise, treat as input data
			s.mu.Lock()
			if s.exec != nil {
				_, _ = s.exec.Write([]byte(msg))
			}
			s.mu.Unlock()
		}
	}()

	<-done
}

func (s *ShellSession) cleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return
	}
	s.closed = true

	if s.exec != nil {
		s.exec.Close()
	}
	s.conn.Close()
}

func (s *ShellSession) sendError(msg string) {
	_ = s.conn.WriteJSON(map[string]string{"error": msg})
}
