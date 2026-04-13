// Copyright (c) 2018, Randy Westlund. All rights reserved.
// This code is under the BSD-2-Clause license.
package quickbooks

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"sync/atomic"
	"time"
)

// Client is your handle to the QuickBooks API.
//
// By default, rate-limited (HTTP 429) responses return a *RateLimitError
// immediately. Call SetMaxRetries to enable automatic retry, and
// SetRetryDelay to configure the wait between attempts (default 1 minute).
type Client struct {
	// Get this from oauth2.NewClient().
	Client *http.Client
	// Set to ProductionEndpoint or SandboxEndpoint.
	endpoint *url.URL
	// The set of quickbooks APIs
	discoveryAPI *DiscoveryAPI
	// The client Id
	clientId string
	// The client Secret
	clientSecret string
	// The minor version of the QB API
	minorVersion string
	// The account Id you're connecting to.
	realmId string

	// Rate-limit state, protected by throttleMu.
	throttleMu     sync.Mutex
	throttledUntil time.Time

	// Maximum number of automatic retries on HTTP 429. Zero (default) means
	// no retries — a *RateLimitError is returned immediately.
	// Accessed atomically.
	maxRetries atomic.Int32

	// Delay between retries on HTTP 429. Defaults to defaultRetryDelay.
	// Stored as nanoseconds; accessed atomically.
	retryDelayNs atomic.Int64
}

// NewClient initializes a new QuickBooks client for interacting with their Online API
func NewClient(clientId string, clientSecret string, realmId string, isProduction bool, minorVersion string, token *BearerToken) (c *Client, err error) {
	if minorVersion == "" {
		minorVersion = "65"
	}

	client := Client{
		clientId:     clientId,
		clientSecret: clientSecret,
		minorVersion: minorVersion,
		realmId:      realmId,
	}

	if isProduction {
		client.endpoint, err = url.Parse(ProductionEndpoint.String() + "/v3/company/" + realmId + "/")
		if err != nil {
			return nil, fmt.Errorf("failed to parse API endpoint: %v", err)
		}

		client.discoveryAPI, err = CallDiscoveryAPI(DiscoveryProductionEndpoint)
		if err != nil {
			return nil, fmt.Errorf("failed to obtain discovery endpoint: %v", err)
		}
	} else {
		client.endpoint, err = url.Parse(SandboxEndpoint.String() + "/v3/company/" + realmId + "/")
		if err != nil {
			return nil, fmt.Errorf("failed to parse API endpoint: %v", err)
		}

		client.discoveryAPI, err = CallDiscoveryAPI(DiscoverySandboxEndpoint)
		if err != nil {
			return nil, fmt.Errorf("failed to obtain discovery endpoint: %v", err)
		}
	}

	if token != nil {
		client.Client = getHttpClient(token)
	}

	return &client, nil
}

// FindAuthorizationUrl compiles the authorization url from the discovery api's auth endpoint.
//
// Example: qbClient.FindAuthorizationUrl("com.intuit.quickbooks.accounting", "security_token", "https://developer.intuit.com/v2/OAuth2Playground/RedirectUrl")
//
// You can find live examples from https://developer.intuit.com/app/developer/playground
func (c *Client) FindAuthorizationUrl(scope string, state string, redirectUri string) (string, error) {
	var authorizationUrl *url.URL

	authorizationUrl, err := url.Parse(c.discoveryAPI.AuthorizationEndpoint)
	if err != nil {
		return "", fmt.Errorf("failed to parse auth endpoint: %v", err)
	}

	urlValues := url.Values{}
	urlValues.Add("client_id", c.clientId)
	urlValues.Add("response_type", "code")
	urlValues.Add("scope", scope)
	urlValues.Add("redirect_uri", redirectUri)
	urlValues.Add("state", state)
	authorizationUrl.RawQuery = urlValues.Encode()

	return authorizationUrl.String(), nil
}

// SetMaxRetries configures the maximum number of automatic retries for
// rate-limited (HTTP 429) requests. The default is 0, meaning no retries —
// a *RateLimitError is returned immediately so callers can handle backoff
// themselves.
//
// When maxRetries > 0, the client sleeps for the configured retry delay
// (see SetRetryDelay, default 1 minute) between attempts. Safe for
// concurrent use.
func (c *Client) SetMaxRetries(n int) {
	if n < 0 {
		n = 0
	}
	c.maxRetries.Store(int32(n))
}

// SetRetryDelay configures the delay between retry attempts when the
// QuickBooks API returns HTTP 429. The default is 1 minute. This delay is
// also used as the throttle window — subsequent requests made before the
// delay elapses will wait for the remaining duration. Safe for concurrent
// use.
func (c *Client) SetRetryDelay(d time.Duration) {
	if d < 0 {
		d = 0
	}
	c.retryDelayNs.Store(int64(d))
}

const defaultRetryDelay = 60 * time.Second

// effectiveRetryDelay returns the configured retry delay or the default.
func (c *Client) effectiveRetryDelay() time.Duration {
	if ns := c.retryDelayNs.Load(); ns > 0 {
		return time.Duration(ns)
	}
	return defaultRetryDelay
}

// waitForThrottle blocks until the current throttle window expires or ctx is
// cancelled, whichever comes first. Returns ctx.Err() if cancelled.
func (c *Client) waitForThrottle(ctx context.Context) error {
	c.throttleMu.Lock()
	until := c.throttledUntil
	c.throttleMu.Unlock()

	d := time.Until(until)
	if d <= 0 {
		return nil
	}

	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// sleepWithContext sleeps for the given duration or until ctx is cancelled.
func sleepWithContext(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// setThrottledUntil records when the throttle expires.
func (c *Client) setThrottledUntil(t time.Time) {
	c.throttleMu.Lock()
	if t.After(c.throttledUntil) {
		c.throttledUntil = t
	}
	c.throttleMu.Unlock()
}

func (c *Client) req(method string, endpoint string, payloadData interface{}, responseObject interface{}, queryParameters map[string]string) error {
	return c.reqCtx(context.Background(), method, endpoint, payloadData, responseObject, queryParameters)
}

func (c *Client) reqCtx(ctx context.Context, method string, endpoint string, payloadData interface{}, responseObject interface{}, queryParameters map[string]string) error {
	endpointUrl := *c.endpoint
	endpointUrl.Path += endpoint
	urlValues := url.Values{}

	if len(queryParameters) > 0 {
		for param, value := range queryParameters {
			urlValues.Add(param, value)
		}
	}

	urlValues.Set("minorversion", c.minorVersion)
	endpointUrl.RawQuery = urlValues.Encode()

	var marshalledJson []byte
	if payloadData != nil {
		var err error
		marshalledJson, err = json.Marshal(payloadData)
		if err != nil {
			return fmt.Errorf("failed to marshal payload: %v", err)
		}
	}

	attempts := 1 + int(c.maxRetries.Load())
	for attempt := 0; attempt < attempts; attempt++ {
		if err := c.waitForThrottle(ctx); err != nil {
			return err
		}

		req, err := http.NewRequestWithContext(ctx, method, endpointUrl.String(), bytes.NewBuffer(marshalledJson))
		if err != nil {
			return fmt.Errorf("failed to create request: %v", err)
		}
		req.Header.Add("Accept", "application/json")
		req.Header.Add("Content-Type", "application/json")

		resp, err := c.Client.Do(req)
		if err != nil {
			return fmt.Errorf("failed to make request: %v", err)
		}

		if resp.StatusCode == http.StatusTooManyRequests {
			delay := c.effectiveRetryDelay()
			c.setThrottledUntil(time.Now().Add(delay))
			resp.Body.Close()

			if attempt+1 < attempts {
				if err := sleepWithContext(ctx, delay); err != nil {
					return err
				}
				continue
			}
			return &RateLimitError{}
		}

		if resp.StatusCode != http.StatusOK {
			err := parseFailure(resp)
			resp.Body.Close()
			return err
		}

		if responseObject != nil {
			if err = json.NewDecoder(resp.Body).Decode(&responseObject); err != nil {
				resp.Body.Close()
				return fmt.Errorf("failed to unmarshal response into object: %v", err)
			}
		}
		resp.Body.Close()
		return nil
	}

	return &RateLimitError{}
}

// doWithThrottle executes an HTTP request with rate-limit awareness and
// optional retry. It is used by paths that cannot go through req() (e.g.
// multipart uploads). The caller provides a buildReq function that constructs
// a fresh *http.Request for each attempt (the body may be consumed on retry).
func (c *Client) doWithThrottle(ctx context.Context, buildReq func() (*http.Request, error)) (*http.Response, error) {
	attempts := 1 + int(c.maxRetries.Load())
	for attempt := 0; attempt < attempts; attempt++ {
		if err := c.waitForThrottle(ctx); err != nil {
			return nil, err
		}

		req, err := buildReq()
		if err != nil {
			return nil, err
		}

		resp, err := c.Client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to make request: %v", err)
		}

		if resp.StatusCode == http.StatusTooManyRequests {
			delay := c.effectiveRetryDelay()
			c.setThrottledUntil(time.Now().Add(delay))
			resp.Body.Close()

			if attempt+1 < attempts {
				if err := sleepWithContext(ctx, delay); err != nil {
					return nil, err
				}
				continue
			}
			return nil, &RateLimitError{}
		}

		return resp, nil
	}

	return nil, &RateLimitError{}
}

func (c *Client) get(endpoint string, responseObject interface{}, queryParameters map[string]string) error {
	return c.req("GET", endpoint, nil, responseObject, queryParameters)
}

func (c *Client) post(endpoint string, payloadData interface{}, responseObject interface{}, queryParameters map[string]string) error {
	return c.req("POST", endpoint, payloadData, responseObject, queryParameters)
}

// query makes the specified QBO `query` and unmarshals the result into `responseObject`
func (c *Client) query(query string, responseObject interface{}) error {
	return c.get("query", responseObject, map[string]string{"query": query})
}
