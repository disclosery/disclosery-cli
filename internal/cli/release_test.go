package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEndpointPrecedence(t *testing.T) {
	for _, tc := range []struct{ name, flag, env, config, want string }{
		{"default", "", "", "", DefaultEndpoint},
		{"config", "", "", "https://config.example", "https://config.example"},
		{"environment", "", "https://env.example", "https://config.example", "https://env.example"},
		{"flag", "https://flag.example", "https://env.example", "https://config.example", "https://flag.example"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("DISCLOSERY_ENDPOINT", tc.env)
			path := filepath.Join(t.TempDir(), "config.json")
			data, _ := json.Marshal(config{Endpoint: tc.config})
			if err := os.WriteFile(path, data, 0600); err != nil {
				t.Fatal(err)
			}
			a := &app{in: strings.NewReader(""), out: &bytes.Buffer{}, err: &bytes.Buffer{}, endpoint: tc.flag, configFile: path, accept: true, ttl: 1}
			cmd := New(a.in, a.out, a.err, "test")
			if err := a.prepare(cmd); err != nil {
				t.Fatal(err)
			}
			if a.client.Origin != tc.want {
				t.Fatalf("origin %q, want %q", a.client.Origin, tc.want)
			}
		})
	}
}

func TestVersion(t *testing.T) {
	var out bytes.Buffer
	cmd := New(strings.NewReader(""), &out, &bytes.Buffer{}, "0.1.0")
	cmd.SetArgs([]string{"--version"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "0.1.0") {
		t.Fatal(out.String())
	}
}
