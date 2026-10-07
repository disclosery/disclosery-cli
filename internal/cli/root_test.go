package cli

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func run(t *testing.T, server string, args ...string) (string, string, error) {
	t.Helper()
	var out, errOut bytes.Buffer
	cmd := New(strings.NewReader(""), &out, &errOut, "test")
	all := []string{"--config", filepath.Join(t.TempDir(), "config.json"), "--endpoint", server, "--accept-terms", "--json"}
	cmd.SetArgs(append(all, args...))
	err := cmd.Execute()
	return out.String(), errOut.String(), err
}
func TestNineCommandsAndKey(t *testing.T) {
	t.Setenv("DISCLOSERY_API_KEY", "test-key")
	cases := []struct {
		args        []string
		path, query string
	}{
		{[]string{"fund", "portfolio", "1922318", "--quarter", "2026q2"}, "/api/v1/funds/1922318/portfolio", "quarter=2026q2"},
		{[]string{"fund", "timeline", "1922318", "LSPD"}, "/api/v1/funds/1922318/timeline/LSPD", ""},
		{[]string{"fund", "filings", "1922318"}, "/api/v1/funds/1922318/filings", ""},
		{[]string{"group", "pale-fire"}, "/api/v1/groups/pale-fire", ""},
		{[]string{"stock", "holders", "LSPD"}, "/api/v1/stocks/LSPD/holders", ""},
		{[]string{"stock", "events", "LSPD"}, "/api/v1/stocks/LSPD/events", ""},
		{[]string{"consensus", "buys"}, "/api/v1/consensus/buys", ""},
		{[]string{"search", "Pale Fire"}, "/api/v1/search", "q=Pale+Fire"},
		{[]string{"filing", "0000921895-26-002694"}, "/api/v1/filings/0000921895-26-002694", ""},
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != tc.path || r.URL.RawQuery != tc.query || r.Header.Get("Authorization") != "Bearer test-key" {
					t.Errorf("request mismatch: %s %s", r.URL.Path, r.URL.RawQuery)
				}
				fmt.Fprint(w, `{"tier":"paid","quota":{"remaining":1999},"data":{"shares":6654851.000000001}}`)
			}))
			defer srv.Close()
			out, notice, e := run(t, srv.URL, tc.args...)
			if e != nil || !strings.Contains(out, "6654851.000000001") || strings.Contains(out, "personal use") || !strings.Contains(notice, "personal use") {
				t.Fatal(out, notice, e)
			}
		})
	}
}
func TestServerErrorsAndCSVGate(t *testing.T) {
	for _, status := range []int{401, 403, 429, 503} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Retry-After", "60")
			w.WriteHeader(status)
			fmt.Fprint(w, `{"error":{"code":"denied","message":"no access"}}`)
		}))
		out, _, e := run(t, srv.URL, "fund", "portfolio", "1922318", "--csv", "--json=false")
		srv.Close()
		if e == nil || out != "" || !strings.Contains(e.Error(), fmt.Sprint(status)) {
			t.Fatal(out, e)
		}
	}
}
func TestWatchDrainsBeforeCheckpoint(t *testing.T) {
	calls := 0
	file := filepath.Join(t.TempDir(), "cursor")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Query().Get("after") != "5" {
			t.Error("lost original cursor")
		}
		if calls == 1 {
			fmt.Fprint(w, `{"data":{"cursor":205,"next_cursor":106,"filings":[{"filing_id":205,"subject":"new"}]}}`)
		} else {
			if r.URL.Query().Get("before") != "106" {
				t.Error("wrong paging")
			}
			fmt.Fprint(w, `{"data":{"cursor":105,"filings":[{"filing_id":6,"subject":"needle"}]}}`)
		}
	}))
	defer srv.Close()
	out, _, e := run(t, srv.URL, "watch", "--after", "5", "--cursor-file", file, "--match", "needle", "--once")
	if e != nil || calls != 2 || !strings.Contains(out, "needle") {
		t.Fatal(out, e, calls)
	}
	b, _ := os.ReadFile(file)
	if string(b) != "205\n" {
		t.Fatalf("cursor %q", b)
	}
}
func TestWatchFailureDoesNotCheckpoint(t *testing.T) {
	file := filepath.Join(t.TempDir(), "cursor")
	os.WriteFile(file, []byte("5\n"), 0600)
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			fmt.Fprint(w, `{"data":{"cursor":205,"next_cursor":106,"filings":[{"filing_id":205}]}}`)
		} else {
			w.WriteHeader(429)
			fmt.Fprint(w, `{"error":{"code":"quota_exceeded","message":"daily quota"}}`)
		}
	}))
	defer srv.Close()
	_, _, e := run(t, srv.URL, "watch", "--cursor-file", file, "--once")
	b, _ := os.ReadFile(file)
	if e == nil || string(b) != "5\n" {
		t.Fatal(e, string(b))
	}
}
func TestCacheBucketsAndTTL(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		fmt.Fprint(w, `{"tier":"paid","quota":{"remaining":12},"data":{}}`)
	}))
	defer srv.Close()
	var stderr bytes.Buffer
	c := &Client{Origin: srv.URL, Key: "secret-a", CacheDir: t.TempDir(), TTL: time.Minute, Err: &stderr}
	for i := 0; i < 2; i++ {
		if _, e := c.get(context.Background(), "/api/v1/search?q=x", false, true); e != nil {
			t.Fatal(e)
		}
	}
	if calls != 1 || !strings.Contains(stderr.String(), "not current") {
		t.Fatal(calls, stderr.String())
	}
	c.Key = "secret-b"
	c.get(context.Background(), "/api/v1/search?q=x", false, true)
	if calls != 2 {
		t.Fatal("cross-key cache")
	}
	files, _ := os.ReadDir(c.CacheDir)
	for _, f := range files {
		b, _ := os.ReadFile(filepath.Join(c.CacheDir, f.Name()))
		if strings.Contains(f.Name(), "secret") || strings.Contains(string(b), "secret") {
			t.Fatal("token leaked")
		}
	}
}
func TestNoConsentInPipe(t *testing.T) {
	var out, stderr bytes.Buffer
	cmd := New(strings.NewReader(""), &out, &stderr, "test")
	cmd.SetArgs([]string{"--endpoint", "http://localhost", "--config", filepath.Join(t.TempDir(), "config.json"), "search", "x"})
	e := cmd.Execute()
	if e == nil || !strings.Contains(e.Error(), "--accept-terms") || out.Len() != 0 {
		t.Fatal(e, out.String())
	}
}
func TestRedirectDoesNotForwardKey(t *testing.T) {
	destCalls := 0
	dest := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { destCalls++ }))
	defer dest.Close()
	src := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, dest.URL, 302) }))
	defer src.Close()
	c := &Client{Origin: src.URL, Key: "secret", Err: &bytes.Buffer{}}
	_, e := c.get(context.Background(), "/api/v1/search?q=x", false, false)
	if e == nil || destCalls != 0 {
		t.Fatal(e, destCalls)
	}
}
func TestConfigKeyAndEmptyEnvOverride(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	var want string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != want {
			t.Errorf("authorization %q", r.Header.Get("Authorization"))
		}
		fmt.Fprint(w, `{"tier":"anonymous","quota":{"remaining":19},"data":{}}`)
	}))
	defer srv.Close()
	os.WriteFile(path, []byte(fmt.Sprintf(`{"endpoint":%q,"api_key":"config-key"}`, srv.URL)), 0600)
	for _, empty := range []bool{false, true} {
		if empty {
			t.Setenv("DISCLOSERY_API_KEY", "")
			want = ""
		} else {
			old, ok := os.LookupEnv("DISCLOSERY_API_KEY")
			os.Unsetenv("DISCLOSERY_API_KEY")
			defer func() {
				if ok {
					os.Setenv("DISCLOSERY_API_KEY", old)
				}
			}()
			want = "Bearer config-key"
		}
		var out, stderr bytes.Buffer
		cmd := New(strings.NewReader(""), &out, &stderr, "test")
		cmd.SetArgs([]string{"--config", path, "--accept-terms", "search", "x"})
		if e := cmd.Execute(); e != nil {
			t.Fatal(e)
		}
	}
}
func TestCSVStreamsAndNeverReadsAnswerCache(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Query().Get("format") != "csv" {
			t.Error("no server CSV request")
		}
		w.Header().Set("Content-Type", "text/csv")
		w.Header().Set("Trailer", "X-Export-Status")
		fmt.Fprint(w, "issuer,shares\nFT,6654851\n")
		w.Header().Set("X-Export-Status", "complete")
	}))
	defer srv.Close()
	for i := 0; i < 2; i++ {
		out, _, e := run(t, srv.URL, "--cache", "fund", "portfolio", "1922318", "--csv", "--json=false")
		if e != nil || out != "issuer,shares\nFT,6654851\n" {
			t.Fatal(e, out)
		}
	}
	if calls != 2 {
		t.Fatal("CSV cached")
	}
}
func TestCSVIncompleteTrailerFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/csv")
		w.Header().Set("Trailer", "X-Export-Status")
		fmt.Fprint(w, "partial,output\n")
		w.Header().Set("X-Export-Status", "incomplete")
	}))
	defer srv.Close()
	out, _, e := run(t, srv.URL, "fund", "portfolio", "1922318", "--csv", "--json=false")
	if e == nil || !strings.Contains(e.Error(), "discard partial") || out == "" {
		t.Fatal(out, e)
	}
}
