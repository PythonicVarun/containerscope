package dockerapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

const DefaultSocketPath = "/var/run/docker.sock"

type Container struct {
	ID     string `json:"id"`
	FullID string `json:"fullId"`
	Name   string `json:"name"`
	Image  string `json:"image"`
	Status string `json:"status"`
	State  string `json:"state"`
}

type LogLine struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type LogStream struct {
	TTY  bool
	Body io.ReadCloser
}

type ExecSession struct {
	ExecID        string
	Conn          net.Conn
	initialData   []byte
	initialOffset int
}

// Read reads from the exec session, first returning any buffered initial data
func (e *ExecSession) Read(p []byte) (int, error) {
	// First, return any remaining initial data
	if e.initialOffset < len(e.initialData) {
		n := copy(p, e.initialData[e.initialOffset:])
		e.initialOffset += n
		return n, nil
	}
	// Then read from the connection
	return e.Conn.Read(p)
}

// Write writes to the exec session
func (e *ExecSession) Write(p []byte) (int, error) {
	return e.Conn.Write(p)
}

// Close closes the exec session
func (e *ExecSession) Close() error {
	return e.Conn.Close()
}

type Client struct {
	httpClient *http.Client
}

type dockerContainerSummary struct {
	ID     string   `json:"Id"`
	Names  []string `json:"Names"`
	Image  string   `json:"Image"`
	Status string   `json:"Status"`
	State  string   `json:"State"`
}

type dockerContainerInfo struct {
	Config struct {
		TTY bool `json:"Tty"`
	} `json:"Config"`
}

func NewClient(socketPath string) *Client {
	transport := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			var dialer net.Dialer
			return dialer.DialContext(ctx, "unix", socketPath)
		},
	}

	return &Client{
		httpClient: &http.Client{Transport: transport},
	}
}

func (c *Client) ListContainers(ctx context.Context) ([]Container, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/containers/json", url.Values{
		"all": []string{"1"},
	})
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, dockerError(resp)
	}

	var containers []dockerContainerSummary
	if err := json.NewDecoder(resp.Body).Decode(&containers); err != nil {
		return nil, err
	}

	result := make([]Container, 0, len(containers))
	for _, container := range containers {
		result = append(result, Container{
			ID:     shortID(container.ID),
			FullID: container.ID,
			Name:   normalizeContainerName(container.Names),
			Image:  container.Image,
			Status: container.Status,
			State:  container.State,
		})
	}

	return result, nil
}

func (c *Client) StartContainer(ctx context.Context, id string) error {
	return c.containerAction(ctx, id, "start")
}

func (c *Client) StopContainer(ctx context.Context, id string) error {
	return c.containerAction(ctx, id, "stop")
}

func (c *Client) RestartContainer(ctx context.Context, id string) error {
	return c.containerAction(ctx, id, "restart")
}

func (c *Client) containerAction(ctx context.Context, id, action string) error {
	req, err := c.newRequestWithBody(ctx, http.MethodPost, "/containers/"+id+"/"+action, nil, nil)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		return dockerError(resp)
	}

	return nil
}

func (c *Client) OpenLogs(ctx context.Context, id string, tail int, follow bool) (*LogStream, error) {
	tty, err := c.containerTTY(ctx, id)
	if err != nil {
		return nil, err
	}

	query := url.Values{
		"stdout":     []string{"1"},
		"stderr":     []string{"1"},
		"timestamps": []string{"1"},
		"tail":       []string{strconv.Itoa(tail)},
	}
	if follow {
		query.Set("follow", "1")
	}

	req, err := c.newRequest(ctx, http.MethodGet, "/containers/"+id+"/logs", query)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		return nil, dockerError(resp)
	}

	return &LogStream{
		TTY:  tty,
		Body: resp.Body,
	}, nil
}

func (c *Client) containerTTY(ctx context.Context, id string) (bool, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/containers/"+id+"/json", nil)
	if err != nil {
		return false, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, dockerError(resp)
	}

	var info dockerContainerInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return false, err
	}

	return info.Config.TTY, nil
}

func (c *Client) newRequest(ctx context.Context, method, path string, query url.Values) (*http.Request, error) {
	return c.newRequestWithBody(ctx, method, path, query, nil)
}

func (c *Client) newRequestWithBody(ctx context.Context, method, path string, query url.Values, body io.Reader) (*http.Request, error) {
	u := &url.URL{
		Scheme:   "http",
		Host:     "docker",
		Path:     path,
		RawQuery: query.Encode(),
	}

	return http.NewRequestWithContext(ctx, method, u.String(), body)
}

func dockerError(resp *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	message := strings.TrimSpace(string(body))
	if message == "" {
		message = resp.Status
	}
	return fmt.Errorf("docker API error (%s): %s", resp.Status, message)
}

func shortID(id string) string {
	if len(id) <= 12 {
		return id
	}
	return id[:12]
}

func normalizeContainerName(names []string) string {
	for _, name := range names {
		trimmed := strings.TrimPrefix(strings.TrimSpace(name), "/")
		if trimmed != "" {
			return trimmed
		}
	}
	return "unknown"
}

// CreateExec creates an exec instance in a container for shell access
func (c *Client) CreateExec(ctx context.Context, containerID string) (string, error) {
	execConfig := map[string]any{
		"AttachStdin":  true,
		"AttachStdout": true,
		"AttachStderr": true,
		"Tty":          true,
		"Cmd":          []string{"/bin/sh", "-i"},
	}

	body, err := json.Marshal(execConfig)
	if err != nil {
		return "", err
	}

	req, err := c.newRequestWithBody(ctx, http.MethodPost, "/containers/"+containerID+"/exec", nil, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return "", dockerError(resp)
	}

	var result struct {
		ID string `json:"Id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	return result.ID, nil
}

// StartExec starts the exec instance and returns a raw connection for bidirectional communication
func (c *Client) StartExec(ctx context.Context, execID string, socketPath string) (*ExecSession, error) {
	execStartConfig := map[string]any{
		"Detach": false,
		"Tty":    true,
	}

	body, err := json.Marshal(execStartConfig)
	if err != nil {
		return nil, err
	}

	// Create raw connection for hijacking
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		return nil, err
	}

	// Build the HTTP request manually for hijacking
	reqPath := fmt.Sprintf("/exec/%s/start", execID)
	httpReq := fmt.Sprintf("POST %s HTTP/1.1\r\n"+
		"Host: docker\r\n"+
		"Content-Type: application/json\r\n"+
		"Connection: Upgrade\r\n"+
		"Upgrade: tcp\r\n"+
		"Content-Length: %d\r\n"+
		"\r\n%s", reqPath, len(body), body)

	if _, err := conn.Write([]byte(httpReq)); err != nil {
		conn.Close()
		return nil, err
	}

	// Read response headers - may need multiple reads to get complete headers
	var fullResp []byte
	buf := make([]byte, 4096)
	headerEnd := -1

	for headerEnd == -1 {
		n, err := conn.Read(buf)
		if err != nil {
			conn.Close()
			return nil, fmt.Errorf("failed to read response: %v", err)
		}
		fullResp = append(fullResp, buf[:n]...)

		// Check if we have the complete headers
		headerEnd = bytes.Index(fullResp, []byte("\r\n\r\n"))

		// Safety limit
		if len(fullResp) > 8192 {
			conn.Close()
			return nil, fmt.Errorf("response headers too large")
		}
	}

	respStr := string(fullResp[:headerEnd])

	// Check for successful upgrade (101 Switching Protocols) or 200 OK
	if !strings.Contains(respStr, "101") && !strings.Contains(respStr, "200") {
		conn.Close()
		return nil, fmt.Errorf("exec start failed: %s", respStr)
	}

	// Create a wrapper that first returns any remaining data after headers
	remainingData := fullResp[headerEnd+4:]

	return &ExecSession{
		ExecID:        execID,
		Conn:          conn,
		initialData:   remainingData,
		initialOffset: 0,
	}, nil
}

// ResizeExec resizes the TTY of an exec instance
func (c *Client) ResizeExec(ctx context.Context, execID string, width, height int) error {
	query := url.Values{
		"w": []string{strconv.Itoa(width)},
		"h": []string{strconv.Itoa(height)},
	}

	req, err := c.newRequestWithBody(ctx, http.MethodPost, "/exec/"+execID+"/resize", query, nil)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return dockerError(resp)
	}

	return nil
}
