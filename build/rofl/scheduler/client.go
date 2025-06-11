package scheduler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/rofl"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/roflmarket"
)

const (
	apiURIAuthLogin = "/rofl-scheduler/v1/auth/login"
	apiURILogsGet   = "/rofl-scheduler/v1/logs/get"
)

// Client is a ROFL scheduler API endpoint client.
type Client struct {
	mu sync.Mutex

	baseURI *url.URL
	hc      *http.Client

	authToken  string
	authExpiry time.Time
}

// NewClient creates a new ROFL scheduler API endpoint client.
func NewClient(dsc *rofl.Registration) (*Client, error) {
	baseURIRaw, ok := dsc.Metadata[MetadataKeySchedulerAPI]
	if !ok {
		return nil, fmt.Errorf("scheduler does not publish an API endpoint")
	}
	baseURI, err := url.Parse(baseURIRaw)
	if err != nil {
		return nil, fmt.Errorf("malformed API endpoint: %w", err)
	}

	hc, err := NewHTTPClient(dsc)
	if err != nil {
		return nil, err
	}

	return &Client{
		baseURI: baseURI,
		hc:      hc,
	}, nil
}

// Host is the hostname.
func (c *Client) Host() string {
	return c.baseURI.Host
}

// Login authenticates to the scheduler using the given login request.
func (c *Client) Login(ctx context.Context, req *AuthLoginRequest) error {
	c.mu.Lock()
	authToken := c.authToken
	authExpiry := c.authExpiry
	c.mu.Unlock()

	if authToken != "" && time.Now().Before(authExpiry) {
		return nil
	}

	var rsp AuthLoginResponse
	if err := c.post(ctx, apiURIAuthLogin, req, &rsp); err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.authToken = rsp.Token
	c.authExpiry = time.Unix(int64(rsp.Expiry), 0) //nolint: gosec
	return nil
}

// LogsGet fetches logs for the given machine.
func (c *Client) LogsGet(ctx context.Context, machineID roflmarket.InstanceID, since time.Time) ([]string, error) {
	hexInstanceID, err := machineID.MarshalText()
	if err != nil {
		return nil, err
	}

	req := LogsGetRequest{
		InstanceID: string(hexInstanceID),
		Since:      uint64(since.Unix()), //nolint: gosec
	}
	var rsp LogsGetResponse
	if err = c.post(ctx, apiURILogsGet, req, &rsp); err != nil {
		return nil, err
	}
	return rsp.Logs, nil
}

func (c *Client) post(ctx context.Context, path string, request, response any) error {
	encRequest, err := json.Marshal(request)
	if err != nil {
		return err
	}

	url := c.baseURI.JoinPath(path).String()
	rq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(encRequest))
	if err != nil {
		return err
	}

	rq.Header.Add("Content-Type", "application/json")

	c.mu.Lock()
	if c.authToken != "" {
		rq.Header.Add("Authorization", "Bearer "+c.authToken)
	}
	c.mu.Unlock()

	rsp, err := c.hc.Do(rq)
	if err != nil {
		return err
	}
	defer rsp.Body.Close()

	data, err := io.ReadAll(rsp.Body)
	if err != nil {
		return err
	}
	switch rsp.StatusCode {
	case http.StatusOK:
		return json.Unmarshal(data, response)
	default:
		return fmt.Errorf("unexpected response from server: %s (%s)", rsp.Status, string(data))
	}
}
