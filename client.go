// Copyright (c) 2018, Randy Westlund. All rights reserved.
// This code is under the BSD-2-Clause license.
package quickbooks

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"
)

// Client is your handle to the QuickBooks API.
//
// By default, rate-limited (HTTP 429) responses return a *RateLimitError
// immediately with the server's Retry-After duration. Call SetMaxRetries to
// enable automatic retry with backoff.
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
	maxRetries int
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
// a *RateLimitError is returned immediately with the server's Retry-After
// duration so callers can handle backoff themselves.
//
// When maxRetries > 0, the client sleeps for the Retry-After duration
// (defaulting to 60s, capped at 2 minutes) between attempts.
func (c *Client) SetMaxRetries(n int) {
	if n < 0 {
		n = 0
	}
	c.maxRetries = n
}

const (
	defaultRetryAfter = 60 * time.Second
	maxRetryAfter     = 2 * time.Minute
)

// waitForThrottle blocks until the current throttle window expires.
func (c *Client) waitForThrottle() {
	c.throttleMu.Lock()
	until := c.throttledUntil
	c.throttleMu.Unlock()

	if d := time.Until(until); d > 0 {
		time.Sleep(d)
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

// parseRetryAfter reads the Retry-After header and returns a clamped duration.
func parseRetryAfter(resp *http.Response) time.Duration {
	raw := resp.Header.Get("Retry-After")
	if raw == "" {
		return defaultRetryAfter
	}
	secs, err := strconv.Atoi(raw)
	if err != nil || secs <= 0 {
		return defaultRetryAfter
	}
	d := time.Duration(secs) * time.Second
	if d > maxRetryAfter {
		d = maxRetryAfter
	}
	return d
}

func (c *Client) req(method string, endpoint string, payloadData interface{}, responseObject interface{}, queryParameters map[string]string) error {
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

	attempts := 1 + c.maxRetries
	for attempt := 0; attempt < attempts; attempt++ {
		c.waitForThrottle()

		req, err := http.NewRequest(method, endpointUrl.String(), bytes.NewBuffer(marshalledJson))
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
			retryAfter := parseRetryAfter(resp)
			c.setThrottledUntil(time.Now().Add(retryAfter))
			resp.Body.Close()

			if attempt+1 < attempts {
				time.Sleep(retryAfter)
				continue
			}
			return &RateLimitError{RetryAfter: retryAfter}
		}

		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return parseFailure(resp)
		}

		if responseObject != nil {
			if err = json.NewDecoder(resp.Body).Decode(&responseObject); err != nil {
				return fmt.Errorf("failed to unmarshal response into object: %v", err)
			}
		}
		return nil
	}

	return &RateLimitError{RetryAfter: defaultRetryAfter}
}

// doWithThrottle executes an HTTP request with rate-limit awareness and
// optional retry. It is used by paths that cannot go through req() (e.g.
// multipart uploads). The caller provides a buildReq function that constructs
// a fresh *http.Request for each attempt (the body may be consumed on retry).
func (c *Client) doWithThrottle(buildReq func() (*http.Request, error)) (*http.Response, error) {
	attempts := 1 + c.maxRetries
	for attempt := 0; attempt < attempts; attempt++ {
		c.waitForThrottle()

		req, err := buildReq()
		if err != nil {
			return nil, err
		}

		resp, err := c.Client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to make request: %v", err)
		}

		if resp.StatusCode == http.StatusTooManyRequests {
			retryAfter := parseRetryAfter(resp)
			c.setThrottledUntil(time.Now().Add(retryAfter))
			resp.Body.Close()

			if attempt+1 < attempts {
				time.Sleep(retryAfter)
				continue
			}
			return nil, &RateLimitError{RetryAfter: retryAfter}
		}

		return resp, nil
	}

	return nil, &RateLimitError{RetryAfter: defaultRetryAfter}
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
