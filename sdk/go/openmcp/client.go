package openmcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"sync"
	"time"
)

// DefaultHTTPTimeout defines the timeout used by clients created without a
// custom http.Client. It is intentionally short to avoid hanging network calls.
const DefaultHTTPTimeout = 15 * time.Second

// Client wraps the HTTP interactions with the OpenMCP Chain REST API.
type Client struct {
	baseURL    *url.URL
	httpClient *http.Client

	mu          sync.RWMutex
	accessToken string
}

// Credentials represents workspace credentials used to obtain access tokens.
type Credentials struct {
	WorkspaceID     string `json:"workspace_id"`
	WorkspaceSecret string `json:"workspace_secret"`
}

// Token represents an issued access token.
type Token struct {
	AccessToken string    `json:"access_token"`
	ExpiresAt   time.Time `json:"expires_at"`
}

// TaskSubmission represents the payload required to create a new task.
type TaskSubmission struct {
	WorkspaceID string                 `json:"workspace_id"`
	Type        string                 `json:"type"`
	Payload     map[string]any         `json:"payload,omitempty"`
	Metadata    map[string]string      `json:"metadata,omitempty"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
}

// TaskSummary contains minimal information about a submitted task.
type TaskSummary struct {
	TaskID      string    `json:"task_id"`
	Status      string    `json:"status"`
	SubmittedAt time.Time `json:"submitted_at"`
}

// TaskDetail contains an extended view of a task.
type TaskDetail struct {
	TaskSummary
	UpdatedAt *time.Time          `json:"updated_at,omitempty"`
	Result    map[string]any      `json:"result,omitempty"`
	Error     *TaskExecutionError `json:"error,omitempty"`
}

// TaskExecutionError represents task level failures.
type TaskExecutionError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// APIError represents server side validation or internal errors.
type APIError struct {
	StatusCode int
	Code       string `json:"code"`
	Message    string `json:"message"`
}

func (e *APIError) Error() string {
	if e == nil {
		return ""
	}
	if e.Code != "" {
		return fmt.Sprintf("openmcp api error (%d): %s - %s", e.StatusCode, e.Code, e.Message)
	}
	return fmt.Sprintf("openmcp api error (%d): %s", e.StatusCode, e.Message)
}

// NewClient instantiates a client for the OpenMCP Chain API. When httpClient is
// nil, a default client with a sensible timeout is used.
func NewClient(rawURL string, httpClient *http.Client) *Client {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		panic(fmt.Sprintf("invalid base url: %v", err))
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: DefaultHTTPTimeout}
	}
	return &Client{baseURL: parsed, httpClient: httpClient}
}

// Authenticate exchanges workspace credentials for an access token and stores it
// for subsequent calls.
func (c *Client) Authenticate(ctx context.Context, creds Credentials) (Token, error) {
	var token Token
	if err := c.post(ctx, "/api/v1/auth/token", creds, &token, false); err != nil {
		return Token{}, err
	}
	c.mu.Lock()
	c.accessToken = token.AccessToken
	c.mu.Unlock()
	return token, nil
}

// SubmitTask creates a new task using the stored access token.
func (c *Client) SubmitTask(ctx context.Context, submission TaskSubmission) (TaskSummary, error) {
	var summary TaskSummary
	if err := c.post(ctx, "/api/v1/tasks", submission, &summary, true); err != nil {
		return TaskSummary{}, err
	}
	return summary, nil
}

// GetTask fetches task details by identifier.
func (c *Client) GetTask(ctx context.Context, taskID string) (TaskDetail, error) {
    var detail TaskDetail
    // 后端通过查询参数 id 获取单个任务
    endpoint := fmt.Sprintf("/api/v1/tasks?id=%s", url.QueryEscape(taskID))
    if err := c.get(ctx, endpoint, &detail, true); err != nil {
        return TaskDetail{}, err
    }
    return detail, nil
}

// AccessToken returns the currently stored token string.
func (c *Client) AccessToken() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.accessToken
}

// SetAccessToken overrides the stored access token.
func (c *Client) SetAccessToken(token string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.accessToken = token
}

func (c *Client) post(ctx context.Context, endpoint string, payload any, out any, withAuth bool) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode request: %w", err)
	}
	req, err := c.newRequest(ctx, http.MethodPost, endpoint, bytes.NewReader(body), withAuth)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	return c.do(req, out)
}

func (c *Client) get(ctx context.Context, endpoint string, out any, withAuth bool) error {
	req, err := c.newRequest(ctx, http.MethodGet, endpoint, nil, withAuth)
	if err != nil {
		return err
	}
	return c.do(req, out)
}

func (c *Client) newRequest(ctx context.Context, method, endpoint string, body io.Reader, withAuth bool) (*http.Request, error) {
	rel := &url.URL{Path: path.Join(c.baseURL.Path, endpoint)}
	u := c.baseURL.ResolveReference(rel)
	req, err := http.NewRequestWithContext(ctx, method, u.String(), body)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	if withAuth {
		token := c.AccessToken()
		if token == "" {
			return nil, errors.New("openmcp: access token is not set")
		}
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return req, nil
}

func (c *Client) do(req *http.Request, out any) error {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("perform request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		var apiErr APIError
		apiErr.StatusCode = resp.StatusCode
		data, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("read error response: %w", err)
		}
		if len(data) > 0 {
			if err := json.Unmarshal(data, &struct {
				Error *APIError `json:"error"`
			}{Error: &apiErr}); err != nil {
				// try direct decode into apiErr if server returned flat payload
				_ = json.Unmarshal(data, &apiErr)
			}
		}
		if apiErr.Message == "" {
			apiErr.Message = string(bytes.TrimSpace(data))
		}
		return &apiErr
	}

	if out == nil {
		return nil
	}
	decoder := json.NewDecoder(resp.Body)
	if err := decoder.Decode(out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}
