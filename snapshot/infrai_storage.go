package snapshot

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const maxAttempts = 4

type Client struct {
	baseURL string
	apiKey  string
	http    *http.Client
	sleep   func(context.Context, time.Duration) error
}

type InfraiError struct {
	Code       string
	Message    string
	HTTPStatus int
}

func (e *InfraiError) Error() string {
	return fmt.Sprintf("Infrai request rejected (%s): %s", e.Code, e.Message)
}

type envelope struct {
	OK       bool            `json:"ok"`
	Data     json.RawMessage `json:"data"`
	Error    *apiError       `json:"error"`
	Metadata json.RawMessage `json:"metadata"`
}

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Hint    string `json:"hint"`
}

func NewClient(baseURL, apiKey string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		http:    httpClient,
		sleep: func(ctx context.Context, delay time.Duration) error {
			timer := time.NewTimer(delay)
			defer timer.Stop()
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-timer.C:
				return nil
			}
		},
	}
}

func (c *Client) StorageBucketCreate(ctx context.Context, name, idempotencyKey string) error {
	// infrai.storage.bucket.create
	body := struct {
		Name           string `json:"name"`
		IdempotencyKey string `json:"idempotency_key"`
	}{name, idempotencyKey}
	return c.call(ctx, http.MethodPost, "/v1/storage/bucket/create", body, nil)
}

func (c *Client) StorageBucketGet(ctx context.Context, name string) error {
	// infrai.storage.bucket.get
	path := "/v1/storage/bucket/get/" + escapeSegment(name)
	return c.call(ctx, http.MethodGet, path, nil, nil)
}

func (c *Client) StorageObjectPut(ctx context.Context, bucket, key string, body objectPutRequest) error {
	// infrai.storage.object.put
	path := "/v1/storage/object/put/" + escapeSegment(bucket) + "/" + escapeKey(key)
	return c.call(ctx, http.MethodPut, path, body, nil)
}

func (c *Client) call(ctx context.Context, method, path string, requestBody, result any) error {
	var payload []byte
	if requestBody != nil {
		var err error
		payload, err = json.Marshal(requestBody)
		if err != nil {
			return fmt.Errorf("encode request: %w", err)
		}
	}
	for attempt := 0; attempt < maxAttempts; attempt++ {
		var body io.Reader
		if payload != nil {
			body = bytes.NewReader(payload)
		}
		req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
		if err != nil {
			return fmt.Errorf("build request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
		req.Header.Set("Content-Type", "application/json")

		res, err := c.http.Do(req)
		if err != nil {
			return fmt.Errorf("send request: %w", err)
		}
		raw, readErr := io.ReadAll(res.Body)
		res.Body.Close()
		if readErr != nil {
			return fmt.Errorf("read response: %w", readErr)
		}

		var env envelope
		decodeErr := json.Unmarshal(raw, &env)
		if decodeErr == nil && !env.OK && env.Error != nil {
			if res.StatusCode == http.StatusTooManyRequests && attempt+1 < maxAttempts {
				if err := c.sleep(ctx, retryDelay(res.Header.Get("Retry-After"), attempt)); err != nil {
					return err
				}
				continue
			}
			message := env.Error.Message
			if message == "" {
				message = env.Error.Hint
			}
			return &InfraiError{Code: env.Error.Code, Message: message, HTTPStatus: res.StatusCode}
		}
		if res.StatusCode >= 500 && attempt+1 < maxAttempts {
			if err := c.sleep(ctx, retryDelay(res.Header.Get("Retry-After"), attempt)); err != nil {
				return err
			}
			continue
		}
		if decodeErr != nil {
			return fmt.Errorf("decode response (HTTP %d): %w", res.StatusCode, decodeErr)
		}
		if !env.OK {
			return fmt.Errorf("request failed with HTTP %d", res.StatusCode)
		}
		if result != nil && len(env.Data) > 0 && string(env.Data) != "null" {
			if err := json.Unmarshal(env.Data, result); err != nil {
				return fmt.Errorf("decode response data: %w", err)
			}
		}
		return nil
	}
	return errors.New("request attempts exhausted")
}

func retryDelay(value string, attempt int) time.Duration {
	if seconds, err := strconv.Atoi(value); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	return time.Duration(1<<attempt) * time.Second
}

func escapeSegment(value string) string {
	return strings.ReplaceAll(urlPathEscape(value), "/", "%2F")
}

func escapeKey(value string) string {
	parts := strings.Split(value, "/")
	for i := range parts {
		parts[i] = urlPathEscape(parts[i])
	}
	return strings.Join(parts, "/")
}

func urlPathEscape(value string) string {
	var builder strings.Builder
	const hex = "0123456789ABCDEF"
	for i := 0; i < len(value); i++ {
		b := value[i]
		if (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') || strings.ContainsRune("-._~", rune(b)) {
			builder.WriteByte(b)
			continue
		}
		builder.WriteByte('%')
		builder.WriteByte(hex[b>>4])
		builder.WriteByte(hex[b&15])
	}
	return builder.String()
}
