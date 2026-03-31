package ws

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"strings"
	"sync"

	"containerscope/internal/dockerapi"
)

type logOpener interface {
	OpenLogs(ctx context.Context, id string, tail int, follow bool) (*dockerapi.LogStream, error)
}

type Session struct {
	conn   *Conn
	docker logOpener

	mu     sync.Mutex
	cancel context.CancelFunc
}

type command struct {
	Action      string `json:"action"`
	ContainerID string `json:"containerId"`
}

type logMessage struct {
	ContainerID string `json:"containerId"`
	Type        string `json:"type"`
	Text        string `json:"text"`
}

func NewSession(conn *Conn, docker logOpener) *Session {
	return &Session{
		conn:   conn,
		docker: docker,
	}
}

func (s *Session) Run() {
	defer s.stopStream()
	defer s.conn.Close()

	for {
		message, err := s.conn.ReadText()
		if err != nil {
			if !errors.Is(err, io.EOF) && !errors.Is(err, net.ErrClosed) {
				log.Printf("websocket read error: %v", err)
			}
			return
		}

		var cmd command
		if err := json.Unmarshal([]byte(message), &cmd); err != nil {
			s.sendError(err)
			continue
		}

		switch cmd.Action {
		case "subscribe":
			if strings.TrimSpace(cmd.ContainerID) == "" {
				s.sendError(fmt.Errorf("containerId is required"))
				continue
			}
			s.subscribe(cmd.ContainerID)
		case "unsubscribe":
			s.stopStream()
		default:
			s.sendError(fmt.Errorf("unsupported action %q", cmd.Action))
		}
	}
}

func (s *Session) subscribe(containerID string) {
	ctx, cancel := context.WithCancel(context.Background())

	s.mu.Lock()
	if s.cancel != nil {
		s.cancel()
	}
	s.cancel = cancel
	s.mu.Unlock()

	go s.streamLogs(ctx, containerID)
}

func (s *Session) stopStream() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}
}

func (s *Session) streamLogs(ctx context.Context, containerID string) {
	stream, err := s.docker.OpenLogs(ctx, containerID, 0, true)
	if err != nil {
		if ctx.Err() == nil {
			s.sendError(err)
		}
		return
	}
	defer stream.Body.Close()

	emit := func(line dockerapi.LogLine) error {
		return s.conn.WriteJSON(logMessage{
			ContainerID: containerID,
			Type:        line.Type,
			Text:        line.Text,
		})
	}

	err = dockerapi.StreamLogs(stream.Body, stream.TTY, emit)
	if err != nil && ctx.Err() == nil && !errors.Is(err, net.ErrClosed) && !isClosedConnectionError(err) {
		s.sendError(err)
	}
}

func (s *Session) sendError(err error) {
	_ = s.conn.WriteJSON(map[string]string{"error": err.Error()})
}

func isClosedConnectionError(err error) bool {
	return strings.Contains(strings.ToLower(err.Error()), "closed network connection")
}
