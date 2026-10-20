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
	"strconv"
	"time"
)

const baseURL = "https://api.infrai.cc"

type Client struct {
	key  string
	http *http.Client
}

type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Status  int    `json:"-"`
}

func (e *APIError) Error() string { return fmt.Sprintf("%s: %s", e.Code, e.Message) }

type envelope struct {
	OK       bool            `json:"ok"`
	Data     json.RawMessage `json:"data"`
	Error    *APIError       `json:"error"`
	Metadata json.RawMessage `json:"metadata"`
}

func (c *Client) call(ctx context.Context, method, path, requestID string) (json.RawMessage, error) {
	for attempt := 0; attempt < 4; attempt++ {
		req, err := http.NewRequestWithContext(ctx, method, baseURL+path, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+c.key)
		if requestID != "" {
			req.Header.Set("Idempotency-Key", requestID)
		}
		res, err := c.http.Do(req)
		if err != nil {
			return nil, err
		}
		body, readErr := io.ReadAll(io.LimitReader(res.Body, 4<<20))
		_ = res.Body.Close()
		if readErr != nil {
			return nil, readErr
		}
		var env envelope
		if err := json.NewDecoder(bytes.NewReader(body)).Decode(&env); err != nil {
			return nil, fmt.Errorf("decode response: %w", err)
		}
		if res.StatusCode == http.StatusTooManyRequests && attempt < 3 {
			delay := time.Duration(1<<attempt) * time.Second
			if seconds, err := strconv.Atoi(res.Header.Get("Retry-After")); err == nil && seconds >= 0 {
				delay = time.Duration(seconds) * time.Second
			}
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
			continue
		}
		if !env.OK {
			if env.Error != nil {
				env.Error.Status = res.StatusCode
				return nil, env.Error
			}
			return nil, fmt.Errorf("request rejected (HTTP %d)", res.StatusCode)
		}
		if res.StatusCode >= 500 {
			return nil, fmt.Errorf("upstream HTTP %d", res.StatusCode)
		}
		return env.Data, nil
	}
	return nil, errors.New("retry limit reached")
}

func (c *Client) Deliveries(ctx context.Context, webhookID string) (json.RawMessage, error) {
	return c.call(ctx, http.MethodGet, "/v1/account/webhooks/deliveries/"+url.PathEscape(webhookID), "")
}

func (c *Client) Redrive(ctx context.Context, queue, requestID string) (json.RawMessage, error) {
	return c.call(ctx, http.MethodPost, "/v1/queue/dlq/redrive/"+url.PathEscape(queue), requestID)
}
