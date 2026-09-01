package tasklist

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"
)

// TaskState represents task states in Camunda Tasklist
type TaskState string

const (
	TaskStateCreated   TaskState = "CREATED"
	TaskStateCompleted TaskState = "COMPLETED"
	TaskStateCanceled  TaskState = "CANCELED"
)

// VariableOperator defines comparison operators for task variable queries
type VariableOperator string

const (
	OpEqual              VariableOperator = "eq"
	OpNotEqual           VariableOperator = "neq"
	OpGreaterThan        VariableOperator = "gt"
	OpGreaterThanOrEqual VariableOperator = "gte"
	OpLessThan           VariableOperator = "lt"
	OpLessThanOrEqual    VariableOperator = "lte"
	OpLike               VariableOperator = "like"
)

// TaskVariableFilter represents variable-based query filter for Tasklist API
type TaskVariableFilter struct {
	Name     string           `json:"name"`
	Value    string           `json:"value"` // JSON string formatted value e.g. "\"GOLD\"" or "5000"
	Operator VariableOperator `json:"operator,omitempty"`
}

// TaskSearchQuery defines the search filters for POST /v1/tasks/search
type TaskSearchQuery struct {
	State                TaskState            `json:"state,omitempty"`
	Assignee             string               `json:"assignee,omitempty"`
	CandidateGroup       string               `json:"candidateGroup,omitempty"`
	CandidateUser        string               `json:"candidateUser,omitempty"`
	ProcessInstanceKey   string               `json:"processInstanceKey,omitempty"`
	ProcessDefinitionKey string               `json:"processDefinitionKey,omitempty"`
	TaskDefinitionID     string               `json:"taskDefinitionId,omitempty"`
	TaskVariables        []TaskVariableFilter `json:"taskVariables,omitempty"`
	PageSize             int                  `json:"pageSize,omitempty"`
}

// Task represents a User Task object returned from Camunda Tasklist REST API
type Task struct {
	ID                   string      `json:"id"`
	Name                 string      `json:"name"`
	TaskDefinitionID     string      `json:"taskDefinitionId"`
	ProcessName          string      `json:"processName"`
	CreationDate         string      `json:"creationDate"`
	CompletionDate       string      `json:"completionDate,omitempty"`
	Assignee             string      `json:"assignee,omitempty"`
	TaskState            TaskState   `json:"taskState"`
	SortValues           []string    `json:"sortValues,omitempty"`
	IsFirst              bool        `json:"isFirst,omitempty"`
	CandidateGroups      []string    `json:"candidateGroups,omitempty"`
	CandidateUsers       []string    `json:"candidateUsers,omitempty"`
	ProcessDefinitionKey string      `json:"processDefinitionKey"`
	ProcessInstanceKey   string      `json:"processInstanceKey"`
	Variables            []Variable  `json:"variables,omitempty"`
}

// Variable represents process variable associated with task
type Variable struct {
	ID    string      `json:"id"`
	Name  string      `json:"name"`
	Value interface{} `json:"value"`
}

// Client provides authenticated access to Camunda 8 Tasklist REST API
type Client struct {
	baseURL    string
	username   string
	password   string
	httpClient *http.Client
}

// NewClient creates a new Tasklist REST Client with cookie jar session management
func NewClient(baseURL, username, password string) (*Client, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create cookie jar: %w", err)
	}

	return &Client{
		baseURL:  strings.TrimRight(baseURL, "/"),
		username: username,
		password: password,
		httpClient: &http.Client{
			Jar:     jar,
			Timeout: 10 * time.Second,
		},
	}, nil
}

// Authenticate performs session login against Tasklist /api/login
func (c *Client) Authenticate(ctx context.Context) error {
	loginURL := fmt.Sprintf("%s/api/login", c.baseURL)
	formData := url.Values{
		"username": {c.username},
		"password": {c.password},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, loginURL, strings.NewReader(formData.Encode()))
	if err != nil {
		return fmt.Errorf("failed to build login request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("login request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusFound && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("authentication failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// SearchTasks queries tasks from the Tasklist Read Model (POST /v1/tasks/search)
func (c *Client) SearchTasks(ctx context.Context, query TaskSearchQuery) ([]Task, error) {
	if query.PageSize <= 0 {
		query.PageSize = 50
	}
	if query.State == "" {
		query.State = TaskStateCreated
	}

	searchURL := fmt.Sprintf("%s/v1/tasks/search", c.baseURL)
	reqBody, err := json.Marshal(query)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal query: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, searchURL, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create search request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("search request error: %w", err)
	}
	defer resp.Body.Close()

	// If unauthorized, re-authenticate and retry once
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		if authErr := c.Authenticate(ctx); authErr != nil {
			return nil, fmt.Errorf("re-auth failed after 401/403: %w", authErr)
		}

		// Retry search
		reqRetry, _ := http.NewRequestWithContext(ctx, http.MethodPost, searchURL, bytes.NewBuffer(reqBody))
		reqRetry.Header.Set("Content-Type", "application/json")
		reqRetry.Header.Set("Accept", "application/json")

		respRetry, retryErr := c.httpClient.Do(reqRetry)
		if retryErr != nil {
			return nil, fmt.Errorf("retry search request failed: %w", retryErr)
		}
		defer respRetry.Body.Close()
		resp = respRetry
	}

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("tasklist API returned error status %d: %s", resp.StatusCode, string(respBody))
	}

	var tasks []Task
	if decErr := json.NewDecoder(resp.Body).Decode(&tasks); decErr != nil {
		return nil, fmt.Errorf("failed to decode task response: %w", decErr)
	}

	return tasks, nil
}

// FetchTaskVariables retrieves all variables for a given task
func (c *Client) FetchTaskVariables(ctx context.Context, taskID string) ([]Variable, error) {
	url := fmt.Sprintf("%s/v1/tasks/%s/variables/search", c.baseURL, taskID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBufferString("{}"))
	if err != nil {
		return nil, fmt.Errorf("failed to create variables search request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("variables search failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("variables API returned status %d: %s", resp.StatusCode, string(body))
	}

	var vars []Variable
	if decErr := json.NewDecoder(resp.Body).Decode(&vars); decErr != nil {
		return nil, fmt.Errorf("failed to decode variables: %w", decErr)
	}

	return vars, nil
}
