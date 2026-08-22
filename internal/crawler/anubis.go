package crawler

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"time"
)

const (
	anubisDriverPath       = "/app/drivers/anubis.mjs"
	anubisDetectionLimit   = 512 * 1024
	anubisDetectionTimeout = 10 * time.Second
	anubisDetectionAgent   = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"
)

func newAnubisDetector() func(context.Context, string) (bool, error) {
	client := &http.Client{Timeout: anubisDetectionTimeout}

	return func(ctx context.Context, targetURL string) (bool, error) {
		req, err := newAnubisDetectionRequest(ctx, targetURL)
		if err != nil {
			return false, err
		}

		resp, err := client.Do(req)
		if err != nil {
			return false, err
		}
		defer resp.Body.Close()

		return responseHasAnubis(resp.Body)
	}
}

func newAnubisDetectionRequest(ctx context.Context, targetURL string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", anubisDetectionAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-Site", "none")
	req.Header.Set("Sec-Fetch-User", "?1")
	req.Header.Set("Upgrade-Insecure-Requests", "1")

	return req, nil
}

func responseHasAnubis(responseBody io.Reader) (bool, error) {
	body, err := io.ReadAll(io.LimitReader(responseBody, anubisDetectionLimit))
	if err != nil {
		return false, err
	}

	body = bytes.ToLower(body)
	if bytes.Contains(body, []byte(`id="anubis_challenge"`)) {
		return true, nil
	}

	return bytes.Contains(body, []byte("/.within.website/x/cmd/anubis/")) &&
		bytes.Contains(body, []byte("github.com/techarohq/anubis")), nil
}
