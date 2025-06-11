package scheduler

import (
	"bytes"
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
	apiUriAuthLogin = "/rofl-scheduler/v1/auth/login"
	apiUriLogsGet   = "/rofl-scheduler/v1/logs/get"
)

// Client is a ROFL scheduler API endpoint client.
type Client struct {
	mu sync.Mutex

	baseUri *url.URL
	hc      *http.Client

	authToken  string
	authExpiry time.Time
}

// NewClient creates a new ROFL scheduler API endpoint client.
func NewClient(dsc *rofl.Registration) (*Client, error) {
	baseUriRaw, ok := dsc.Metadata[MetadataKeySchedulerAPI]
	if !ok {
		return nil, fmt.Errorf("scheduler does not publish an API endpoint")
	}
	baseUri, err := url.Parse(baseUriRaw)
	if err != nil {
		return nil, fmt.Errorf("malformed API endpoint: %w", err)
	}

	hc, err := NewHTTPClient(dsc)
	if err != nil {
		return nil, err
	}

	return &Client{
		baseUri: baseUri,
		hc:      hc,
	}, nil
}

// Host is the hostname.
func (c *Client) Host() string {
	return c.baseUri.Host
}

// Login authenticates to the scheduler using the given login request.
func (c *Client) Login(req *AuthLoginRequest) error {
	c.mu.Lock()
	authToken := c.authToken
	authExpiry := c.authExpiry
	c.mu.Unlock()

	if authToken != "" && time.Now().Before(authExpiry) {
		return nil
	}

	var rsp AuthLoginResponse
	if err := c.post(apiUriAuthLogin, req, &rsp); err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.authToken = rsp.Token
	c.authExpiry = time.Unix(int64(rsp.Expiry), 0)
	return nil
}

// LogsGet fetches logs for the given machine.
func (c *Client) LogsGet(machineID roflmarket.InstanceID, since time.Time) ([]string, error) {
	hexInstanceID, err := machineID.MarshalText()
	if err != nil {
		return nil, err
	}

	req := LogsGetRequest{
		InstanceID: string(hexInstanceID),
		Since:      uint64(since.Unix()),
	}
	var rsp LogsGetResponse
	if err = c.post(apiUriLogsGet, req, &rsp); err != nil {
		return nil, err
	}
	return rsp.Logs, nil
}

func (c *Client) post(path string, request, response any) error {
	encRequest, err := json.Marshal(request)
	if err != nil {
		return err
	}

	url := c.baseUri.JoinPath(path).String()
	rq, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(encRequest))
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
