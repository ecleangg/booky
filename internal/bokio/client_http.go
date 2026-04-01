package bokio

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
)

func (c *Client) doJSON(ctx context.Context, method, endpoint string, payload any, out any) error {
	fullURL, err := c.resolveURL(endpoint)
	if err != nil {
		return err
	}

	var body io.Reader
	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("marshal bokio payload: %w", err)
		}
		body = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, fullURL, body)
	if err != nil {
		return fmt.Errorf("create bokio request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("call bokio api: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read bokio response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("bokio api %s returned %d: %s", endpoint, resp.StatusCode, bytes.TrimSpace(bodyBytes))
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(bodyBytes, out); err != nil {
		return fmt.Errorf("decode bokio response: %w", err)
	}
	return nil
}

func (c *Client) resolveURL(endpoint string) (string, error) {
	base, err := url.Parse(c.baseURL)
	if err != nil {
		return "", fmt.Errorf("parse bokio base url: %w", err)
	}
	ref, err := url.Parse(strings.TrimLeft(endpoint, "/"))
	if err != nil {
		return "", fmt.Errorf("parse bokio endpoint: %w", err)
	}
	base.Path = path.Join(base.Path, ref.Path)
	base.RawQuery = ref.RawQuery
	return base.String(), nil
}
