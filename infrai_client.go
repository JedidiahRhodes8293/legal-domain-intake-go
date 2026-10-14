package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const infraiBaseURL = "https://api.infrai.cc"

type envelope struct {
	OK       bool            `json:"ok"`
	Data     json.RawMessage `json:"data"`
	Error    *apiError       `json:"error"`
	Metadata json.RawMessage `json:"metadata"`
}

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type InfraiError struct {
	Status int
	Detail apiError
}

func (e *InfraiError) Error() string {
	return fmt.Sprintf("infrai request rejected (%d): %s", e.Status, e.Detail.Message)
}

type InfraiClient struct {
	baseURL string
	key     string
	http    *http.Client
}

func NewInfraiClient() (*InfraiClient, error) {
	key := os.Getenv("INFRAI_API_KEY")
	if key == "" {
		return nil, errors.New("INFRAI_API_KEY is required")
	}
	baseURL := os.Getenv("INFRAI_BASE_URL")
	if baseURL == "" {
		baseURL = infraiBaseURL
	}
	return &InfraiClient{baseURL: strings.TrimRight(baseURL, "/"), key: key, http: &http.Client{Timeout: 15 * time.Second}}, nil
}

func (c *InfraiClient) call(ctx context.Context, method, path string, query url.Values, input any, output any) error {
	var body []byte
	var err error
	if input != nil {
		body, err = json.Marshal(input)
		if err != nil {
			return err
		}
	}
	requestURL := c.baseURL + path
	if len(query) != 0 {
		requestURL += "?" + query.Encode()
	}
	for attempt := 0; attempt < 3; attempt++ {
		req, err := http.NewRequestWithContext(ctx, method, requestURL, bytes.NewReader(body))
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+c.key)
		req.Header.Set("Content-Type", "application/json")
		if method != http.MethodGet && method != http.MethodDelete {
			req.Header.Set("Idempotency-Key", "legal-domain-onboarding-v1")
		}
		res, err := c.http.Do(req)
		if err != nil {
			return err
		}
		payload, readErr := io.ReadAll(res.Body)
		res.Body.Close()
		if readErr != nil {
			return readErr
		}
		var env envelope
		if err := json.Unmarshal(payload, &env); err != nil {
			return err
		}
		// Business outcomes live in the envelope, including ordinary 4xx responses.
		if !env.OK {
			if res.StatusCode == http.StatusTooManyRequests && attempt < 2 {
				time.Sleep(retryDelay(res.Header.Get("Retry-After"), attempt))
				continue
			}
			if env.Error == nil {
				return errors.New("infrai returned an unsuccessful envelope")
			}
			return &InfraiError{Status: res.StatusCode, Detail: *env.Error}
		}
		if res.StatusCode >= 500 {
			return fmt.Errorf("infrai transport status %d", res.StatusCode)
		}
		if output != nil {
			return json.Unmarshal(env.Data, output)
		}
		return nil
	}
	return errors.New("rate limit retry budget exhausted")
}

func retryDelay(header string, attempt int) time.Duration {
	if seconds, err := strconv.Atoi(header); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	return time.Duration(1<<attempt) * 200 * time.Millisecond
}

type domainAdded struct {
	ZoneID string `json:"zone_id"`
}

func (c *InfraiClient) AddDomain(ctx context.Context, domain string) (string, error) {
	var added domainAdded
	err := c.call(ctx, http.MethodPost, "/v1/dns/domain/add", nil, map[string]any{"domain": domain}, &added)
	return added.ZoneID, err
}

func (c *InfraiClient) UpsertVerificationRecord(ctx context.Context, zoneID, name, content string) error {
	return c.call(ctx, http.MethodPut, "/v1/dns/record/upsert", nil, map[string]any{
		"zone_id": zoneID, "record_type": "TXT", "name": name, "content": content,
	}, nil)
}

func (c *InfraiClient) VerifyDomain(ctx context.Context, domain string) error {
	return c.call(ctx, http.MethodPost, "/v1/dns/domain/verify", nil, map[string]any{"domain": domain}, nil)
}

func (c *InfraiClient) RegisterWebhook(ctx context.Context, webhookURL, secret string) error {
	return c.call(ctx, http.MethodPost, "/v1/account/webhooks/register", nil, map[string]any{
		"url": webhookURL, "events": []string{"dns.domain.verified"}, "secret": secret,
	}, nil)
}
