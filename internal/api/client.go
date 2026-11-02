// Package api is the client for the Pulseczar v1 JSON API.
package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// Client calls one Czar. Token is empty only for the sign-in routes.
type Client struct {
	BaseURL string
	Token   string
	HTTP    *http.Client
}

// call sends one request and decodes the body into T. It also returns the body
// exactly as the server sent it, so --json can print it unchanged.
func call[T any](c *Client, method, path string, query url.Values, in any) (T, []byte, error) {
	var out T

	raw, status, err := c.send(method, path, query, in)
	if err != nil {
		return out, nil, err
	}

	if status >= 300 {
		return out, raw, newError(status, raw)
	}

	if len(raw) == 0 {
		return out, raw, nil
	}

	if err := json.Unmarshal(raw, &out); err != nil {
		return out, raw, fmt.Errorf("the server sent a body that is not valid JSON: %w", err)
	}

	return out, raw, nil
}

func (c *Client) send(method, path string, query url.Values, in any) ([]byte, int, error) {
	target := strings.TrimRight(c.BaseURL, "/") + "/api/v1/" + path
	if len(query) > 0 {
		target += "?" + query.Encode()
	}

	var body io.Reader
	if in != nil {
		encoded, err := json.Marshal(in)
		if err != nil {
			return nil, 0, err
		}
		body = bytes.NewReader(encoded)
	}

	request, err := http.NewRequest(method, target, body)
	if err != nil {
		return nil, 0, err
	}
	request.Header.Set("Accept", "application/json")
	if in != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if c.Token != "" {
		request.Header.Set("Authorization", "Bearer "+c.Token)
	}

	response, err := c.HTTP.Do(request)
	if err != nil {
		return nil, 0, &NetworkError{Err: err}
	}
	defer response.Body.Close()

	raw, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, 0, &NetworkError{Err: err}
	}

	return raw, response.StatusCode, nil
}
