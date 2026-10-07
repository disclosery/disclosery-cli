package cli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const maxResponse = 16 << 20

// Client never reads the site module, database or providers.
type Client struct {
	Origin, Key, CacheDir string
	TTL                   time.Duration
	HTTP                  *http.Client
	Err                   io.Writer
}
type cached struct {
	At   time.Time       `json:"at"`
	Body json.RawMessage `json:"body"`
}

func (c *Client) get(ctx context.Context, path string, csv, cache bool) ([]byte, error) {
	u, err := url.Parse(c.Origin)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Scheme != "https" && !(u.Scheme == "http" && (u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1" || u.Hostname() == "::1"))) {
		return nil, fmt.Errorf("endpoint must be HTTPS or loopback HTTP, without credentials, query or fragment")
	}
	u.Path = strings.TrimRight(u.Path, "/")
	address := strings.TrimRight(u.String(), "/") + path
	digest := sha256.Sum256([]byte(c.Origin + "\x00" + c.Key + "\x00" + path))
	file := filepath.Join(c.CacheDir, hex.EncodeToString(digest[:])+".json")
	if cache && !csv && c.TTL > 0 {
		if b, e := os.ReadFile(file); e == nil {
			var v cached
			if json.Unmarshal(b, &v) == nil && time.Since(v.At) >= 0 && time.Since(v.At) < c.TTL && json.Valid(v.Body) {
				fmt.Fprintf(c.Err, "Cached answer (%s old); quota is from the original response, not current.\n", time.Since(v.At).Round(time.Second))
				return v.Body, nil
			}
		}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return nil, fmt.Errorf("invalid API request")
	}
	if c.Key != "" {
		req.Header.Set("Authorization", "Bearer "+c.Key)
	}
	if csv {
		req.Header.Set("Accept", "text/csv")
	} else {
		req.Header.Set("Accept", "application/json")
	}
	req.Header.Set("User-Agent", "Disclosery-CLI/1")
	h := c.HTTP
	if h == nil {
		h = &http.Client{Timeout: 20 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	}
	resp, err := h.Do(req)
	if err != nil {
		return nil, fmt.Errorf("API request failed (endpoint unavailable or request cancelled)")
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, maxResponse+1))
	if err != nil {
		return nil, fmt.Errorf("reading API response failed")
	}
	if len(b) > maxResponse {
		return nil, fmt.Errorf("API response exceeds 16 MiB")
	}
	if resp.StatusCode != http.StatusOK {
		var v struct {
			Error struct {
				Code, Message string
				ResetAt       string `json:"reset_at"`
			} `json:"error"`
		}
		if json.Unmarshal(b, &v) == nil && v.Error.Code != "" {
			return nil, fmt.Errorf("API %d %s: %s (Retry-After: %s)", resp.StatusCode, v.Error.Code, redact(v.Error.Message, c.Key), resp.Header.Get("Retry-After"))
		}
		return nil, fmt.Errorf("API returned HTTP %d", resp.StatusCode)
	}
	if csv {
		if !strings.HasPrefix(resp.Header.Get("Content-Type"), "text/csv") {
			return nil, fmt.Errorf("API did not return server-authorized CSV")
		}
		return b, nil
	}
	if !json.Valid(b) {
		return nil, fmt.Errorf("API returned invalid JSON")
	}
	if cache && c.TTL > 0 {
		if err = os.MkdirAll(c.CacheDir, 0700); err != nil {
			return nil, err
		}
		raw, _ := json.Marshal(cached{time.Now(), b})
		if err = atomicWrite(file, raw); err != nil {
			return nil, err
		}
	}
	return b, nil
}
func atomicWrite(path string, b []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".disclosery-")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if err = f.Chmod(0600); err == nil {
		_, err = f.Write(b)
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(f.Name(), path)
}

// download streams a server-authorized export and always performs fresh authentication.
func (c *Client) download(ctx context.Context, path string, out io.Writer) error {
	u, err := url.Parse(c.Origin)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Scheme != "https" && !(u.Scheme == "http" && (u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1" || u.Hostname() == "::1"))) {
		return fmt.Errorf("endpoint must be HTTPS or loopback HTTP, without credentials, query or fragment")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(c.Origin, "/")+path, nil)
	if err != nil {
		return fmt.Errorf("invalid API request")
	}
	req.Header.Set("Accept", "text/csv")
	req.Header.Set("User-Agent", "Disclosery-CLI/1")
	if c.Key != "" {
		req.Header.Set("Authorization", "Bearer "+c.Key)
	}
	h := c.HTTP
	if h == nil {
		h = &http.Client{Timeout: 2 * time.Minute, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	}
	resp, err := h.Do(req)
	if err != nil {
		return fmt.Errorf("CSV request failed (endpoint unavailable or request cancelled)")
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 65536))
		var v struct {
			Error struct{ Code, Message string } `json:"error"`
		}
		if json.Unmarshal(b, &v) == nil && v.Error.Code != "" {
			return fmt.Errorf("API %d %s: %s (Retry-After: %s)", resp.StatusCode, v.Error.Code, redact(v.Error.Message, c.Key), resp.Header.Get("Retry-After"))
		}
		return fmt.Errorf("API returned HTTP %d", resp.StatusCode)
	}
	if !strings.HasPrefix(resp.Header.Get("Content-Type"), "text/csv") {
		return fmt.Errorf("API did not return server-authorized CSV")
	}
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return fmt.Errorf("CSV stream interrupted; discard partial output")
	}
	if resp.Trailer.Get("X-Export-Status") != "complete" {
		return fmt.Errorf("CSV export incomplete; discard partial output")
	}
	fmt.Fprintf(c.Err, "CSV quota remaining: %s (reset %s)\n", resp.Header.Get("X-Quota-Remaining"), resp.Header.Get("X-Quota-Reset"))
	return nil
}

func redact(message, key string) string {
	if key != "" {
		return strings.ReplaceAll(message, key, "[redacted]")
	}
	return message
}
