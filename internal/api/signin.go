package api

import (
	"encoding/json"
	"net/http"
)

// The device authorization grant of RFC 8628, as Czar serves it.
const deviceCodeGrant = "urn:ietf:params:oauth:grant-type:device_code"

// The RFC 8628 words a poll answers with before the token is ready.
const (
	PollPending  = "authorization_pending"
	PollSlowDown = "slow_down"
	PollDenied   = "access_denied"
	PollExpired  = "expired_token"
)

// SignInStart is what Czar answers when the CLI opens a sign-in.
type SignInStart struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

// PollResult is one poll's answer: the token once approved, otherwise the RFC word.
type PollResult struct {
	Token string
	Error string
}

// StartSignIn opens a sign-in for a device and returns the code the person approves on the web.
func (c *Client) StartSignIn(deviceName string) (SignInStart, error) {
	start, _, err := call[SignInStart](c, http.MethodPost, "auth/cli", nil, map[string]string{
		"device_name": deviceName,
	})

	return start, err
}

// PollSignIn asks once whether the person has answered. A 400 is an answer, not a failure.
func (c *Client) PollSignIn(deviceCode string) (PollResult, error) {
	raw, status, err := c.send(http.MethodPost, "auth/cli/token", nil, map[string]string{
		"grant_type":  deviceCodeGrant,
		"device_code": deviceCode,
	})
	if err != nil {
		return PollResult{}, err
	}

	var body struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
	}
	_ = json.Unmarshal(raw, &body)

	if status == http.StatusOK && body.AccessToken != "" {
		return PollResult{Token: body.AccessToken}, nil
	}

	if status == http.StatusBadRequest && body.Error != "" {
		return PollResult{Error: body.Error}, nil
	}

	return PollResult{}, newError(status, raw)
}

// Logout revokes the token the client sends.
func (c *Client) Logout() error {
	_, _, err := call[struct{}](c, http.MethodPost, "auth/logout", nil, nil)

	return err
}
