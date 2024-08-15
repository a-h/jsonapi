package jsonapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

type Doer interface {
	Do(req *http.Request) (*http.Response, error)
}

type Config struct {
	URL        *url.URL
	Client     Doer
	Middleware []Middleware
}

type Middleware interface {
	Request(req *http.Request) error
	Response(res *http.Response) error
}

// WithTimeout attempts to set the timeout on the HTTP client.
// It is a no-op if the underlying Doer is not an *http.Client.
func WithTimeout(timeout time.Duration) Opt {
	return func(c *Config) error {
		if c.Client == nil {
			c.Client = http.DefaultClient
		}
		if httpc, ok := c.Client.(*http.Client); ok {
			httpc.Timeout = timeout
		}
		return nil
	}
}

// WithClient uses a custom Doer for the HTTP requests.
// Typically, this is a *http.Client.
func WithClient(client Doer) Opt {
	return func(c *Config) error {
		c.Client = client
		return nil
	}
}

// WithMiddleware adds middleware to the HTTP request.
// See the github.com/a-h/jsonapi/middleware package for middleware.
func WithMiddleware(middleware ...Middleware) Opt {
	return func(c *Config) error {
		c.Middleware = append(c.Middleware, middleware...)
		return nil
	}
}

// Opt is an option for the JSON API client.
// See WithTimeout, WithClient, and WithMiddleware.
type Opt func(*Config) (err error)

func newConfig(u string, opts ...Opt) (*Config, error) {
	pu, err := url.Parse(u)
	if err != nil {
		return nil, fmt.Errorf("failed to parse URL: %w", err)
	}
	c := &Config{
		URL:    pu,
		Client: http.DefaultClient,
		Middleware: []Middleware{
			&requestHeaderMiddleware{"Content-Type", "application/json"},
		},
	}
	for _, o := range opts {
		if err := o(c); err != nil {
			return nil, fmt.Errorf("failed to apply option: %w", err)
		}
	}
	return c, nil
}

// Post a HTTP request to the given URL with the given request body.
func Post[TReq, TResp any](ctx context.Context, url string, request TReq, opts ...Opt) (response TResp, err error) {
	config, err := newConfig(url, opts...)
	if err != nil {
		return response, fmt.Errorf("failed to create config: %w", err)
	}
	buf, err := json.Marshal(request)
	if err != nil {
		return response, fmt.Errorf("failed to marshal request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(buf))
	if err != nil {
		return response, fmt.Errorf("failed to create request: %w", err)
	}
	for _, m := range config.Middleware {
		if err := m.Request(req); err != nil {
			return response, fmt.Errorf("middleware failed to modify request: %w", err)
		}
	}
	res, err := config.Client.Do(req)
	if err != nil {
		return response, fmt.Errorf("failed to perform HTTP request: %w", err)
	}
	for _, m := range config.Middleware {
		if err := m.Response(res); err != nil {
			return response, fmt.Errorf("middleware failed to modify response: %w", err)
		}
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode > 299 {
		body, _ := io.ReadAll(res.Body)
		return response, InvalidStatusError{
			Status: res.StatusCode,
			Body:   string(body),
		}
	}
	bodyBytes, err := io.ReadAll(res.Body)
	if err != nil {
		return response, fmt.Errorf("failed to read response body: %w", err)
	}
	if err := json.Unmarshal(bodyBytes, &response); err != nil {
		return response, InvalidJSONError{
			Status: res.StatusCode,
			Body:   string(bodyBytes),
			Err:    err,
		}
	}
	return response, nil
}

// Get a HTTP response from the given URL.
// Returns ok=false if the response was a 404.
func Get[TResp any](ctx context.Context, url string, opts ...Opt) (response TResp, ok bool, err error) {
	config, err := newConfig(url, opts...)
	if err != nil {
		return response, false, fmt.Errorf("failed to create config: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return response, false, fmt.Errorf("failed to create request: %w", err)
	}
	for _, m := range config.Middleware {
		if err := m.Request(req); err != nil {
			return response, false, fmt.Errorf("middleware failed to modify request: %w", err)
		}
	}
	res, err := config.Client.Do(req)
	if err != nil {
		return response, false, fmt.Errorf("failed to perform HTTP request: %w", err)
	}
	defer res.Body.Close()
	for _, m := range config.Middleware {
		if err := m.Response(res); err != nil {
			return response, false, fmt.Errorf("middleware failed to modify response: %w", err)
		}
	}
	if res.StatusCode == http.StatusNotFound {
		return response, false, nil
	}
	if res.StatusCode < 200 || res.StatusCode > 299 {
		body, _ := io.ReadAll(res.Body)
		return response, false, InvalidStatusError{
			Status: res.StatusCode,
			Body:   string(body),
		}
	}
	bodyBytes, err := io.ReadAll(res.Body)
	if err != nil {
		return response, false, fmt.Errorf("failed to read response body: %w", err)
	}
	if err := json.Unmarshal(bodyBytes, &response); err != nil {
		return response, false, InvalidJSONError{
			Status: res.StatusCode,
			Body:   string(bodyBytes),
			Err:    err,
		}
	}
	return response, true, nil
}

type InvalidStatusError struct {
	Status int    `json:"status"`
	Body   string `json:"body"`
}

func (e InvalidStatusError) Error() string {
	return fmt.Sprintf("api responded with non-success status %d: message: %s", e.Status, e.Body)
}

type InvalidJSONError struct {
	Status int    `json:"status"`
	Body   string `json:"body"`
	Err    error  `json:"error"`
}

func (e InvalidJSONError) Error() string {
	return fmt.Sprintf("api responded with 2xx status code %d, but the response could not be decoded with error: %v: %q", e.Status, e.Err, e.Body)
}