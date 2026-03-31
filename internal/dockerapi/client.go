package dockerapi

import (
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
		"all": []string{"0"},
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
	u := &url.URL{
		Scheme:   "http",
		Host:     "docker",
		Path:     path,
		RawQuery: query.Encode(),
	}

	return http.NewRequestWithContext(ctx, method, u.String(), nil)
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
