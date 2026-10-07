package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"github.com/spf13/cobra"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const DefaultEndpoint = "https://disclosery.com"

const terms = "Disclosery is for personal use. Follow the service terms at your configured endpoint /terms and read /data and /methods for coverage and estimates. CSV export requires server-authorized paid access."

type config struct {
	Endpoint string `json:"endpoint"`
	APIKey   string `json:"api_key"`
}
type app struct {
	in                             io.Reader
	out, err                       io.Writer
	endpoint, configFile, cacheDir string
	json, accept, cache            bool
	ttl                            time.Duration
	client                         *Client
}

func New(in io.Reader, out, stderr io.Writer, version string) *cobra.Command {
	home, _ := os.UserHomeDir()
	a := &app{in: in, out: out, err: stderr, configFile: filepath.Join(home, ".config", "disclosery", "config.json"), cacheDir: filepath.Join(home, ".cache", "disclosery")}
	root := &cobra.Command{Use: "disclosery", Short: "Query stored SEC ownership filings through Disclosery API v1", Version: version, SilenceUsage: true, SilenceErrors: true}
	root.SetIn(in)
	root.SetOut(out)
	root.SetErr(stderr)
	root.PersistentFlags().StringVar(&a.endpoint, "endpoint", "", "API origin (default https://disclosery.com; flag overrides environment and config)")
	root.PersistentFlags().StringVar(&a.configFile, "config", a.configFile, "JSON config containing endpoint and api_key")
	root.PersistentFlags().BoolVar(&a.json, "json", false, "Print exact API JSON, retaining decimal precision")
	root.PersistentFlags().BoolVar(&a.accept, "accept-terms", false, "Acknowledge personal-use notice without a prompt (cron/pipes)")
	root.PersistentFlags().BoolVar(&a.cache, "cache", false, "Reuse answers within --cache-ttl; quota in cached answers is historical")
	root.PersistentFlags().DurationVar(&a.ttl, "cache-ttl", 5*time.Minute, "Maximum cached answer age")
	root.PersistentPreRunE = func(cmd *cobra.Command, args []string) error { return a.prepare(cmd) }
	fund := &cobra.Command{Use: "fund", Short: "Fund holdings, timelines and filings"}
	stock := &cobra.Command{Use: "stock", Short: "Stock holders and ownership events"}
	root.AddCommand(fund, stock)
	a.query(fund, "portfolio CIK", "Quarterly fund holdings", "/api/v1/funds/%s/portfolio", 1, []string{"quarter", "table", "page"}, true)
	a.query(fund, "timeline CIK SECURITY", "Position history for one ticker or CUSIP", "/api/v1/funds/%s/timeline/%s", 2, nil, false)
	a.query(fund, "filings CIK", "Fund filing history", "/api/v1/funds/%s/filings", 1, []string{"since", "limit"}, false)
	a.query(root, "group SLUG", "Related filers and their filing union", "/api/v1/groups/%s", 1, []string{"limit"}, false)
	a.query(stock, "holders TICKER", "Manager-collapsed holders", "/api/v1/stocks/%s/holders", 1, []string{"quarter", "page"}, false)
	a.query(stock, "events TICKER", "Ownership event feed", "/api/v1/stocks/%s/events", 1, []string{"since", "limit"}, false)
	a.query(root, "consensus KIND", "Institutional rankings", "/api/v1/consensus/%s", 1, []string{"quarter"}, false)
	a.query(root, "search QUERY", "Find funds, groups, stocks, insiders and filings", "/api/v1/search", 1, []string{"limit"}, false)
	a.query(root, "filing ACCESSION", "One SEC filing", "/api/v1/filings/%s", 1, []string{"page"}, false)
	root.AddCommand(a.watch())
	notice := &cobra.Command{Use: "terms", Short: "Display the personal-use notice", RunE: func(cmd *cobra.Command, args []string) error { _, err := fmt.Fprintln(out, terms); return err }, Annotations: map[string]string{"skipSetup": "true"}}
	root.AddCommand(notice)
	return root
}
func (a *app) prepare(cmd *cobra.Command) error {
	if cmd.Annotations["skipSetup"] == "true" {
		return nil
	}
	if a.ttl <= 0 || a.ttl > 24*time.Hour {
		return fmt.Errorf("cache TTL must be positive and at most 24h")
	}
	var cfg config
	b, err := os.ReadFile(a.configFile)
	if err == nil {
		if json.Unmarshal(b, &cfg) != nil {
			return fmt.Errorf("invalid config JSON")
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("cannot read config")
	}
	key := cfg.APIKey
	if v, ok := os.LookupEnv("DISCLOSERY_API_KEY"); ok {
		key = v
	}
	origin := a.endpoint
	if origin == "" {
		origin = os.Getenv("DISCLOSERY_ENDPOINT")
	}
	if origin == "" {
		origin = cfg.Endpoint
	}
	if origin == "" {
		origin = DefaultEndpoint
	}
	consent := filepath.Join(filepath.Dir(a.configFile), "personal-use-notice-v1")
	if _, err = os.Stat(consent); err != nil {
		fmt.Fprintln(a.err, terms)
		if !a.accept {
			f, ok := a.in.(*os.File)
			if !ok {
				return fmt.Errorf("first use requires --accept-terms for automation")
			}
			st, e := f.Stat()
			if e != nil || st.Mode()&os.ModeCharDevice == 0 {
				return fmt.Errorf("first use requires --accept-terms for automation")
			}
			fmt.Fprint(a.err, "Acknowledge personal-use notice? [y/N] ")
			answer, e := bufio.NewReader(a.in).ReadString('\n')
			if e != nil || !strings.EqualFold(strings.TrimSpace(answer), "y") {
				return fmt.Errorf("personal-use notice not acknowledged")
			}
		}
		if err = atomicWrite(consent, []byte("personal-use-notice-v1\n")); err != nil {
			return err
		}
	}
	a.client = &Client{Origin: origin, Key: key, CacheDir: a.cacheDir, TTL: a.ttl, Err: a.err}
	return nil
}
func (a *app) query(parent *cobra.Command, use, short, path string, count int, flags []string, holdings bool) {
	values := map[string]*string{}
	csv := false
	cmd := &cobra.Command{Use: use, Short: short, Args: cobra.ExactArgs(count), RunE: func(cmd *cobra.Command, args []string) error {
		if csv && a.json {
			return fmt.Errorf("--csv and --json are mutually exclusive")
		}
		if csv && values["table"] != nil && *values["table"] != "" {
			return fmt.Errorf("CSV exports holdings only")
		}
		escaped := make([]any, len(args))
		for i, v := range args {
			escaped[i] = url.PathEscape(v)
		}
		route := path
		if strings.Contains(path, "%s") {
			route = fmt.Sprintf(path, escaped...)
		}
		q := url.Values{}
		if path == "/api/v1/search" {
			q.Set("q", args[0])
		}
		for k, v := range values {
			if *v != "" {
				q.Set(k, *v)
			}
		}
		if csv {
			q.Set("format", "csv")
		}
		if len(q) > 0 {
			route += "?" + q.Encode()
		}
		if csv {
			return a.client.download(cmd.Context(), route, a.out)
		}
		body, err := a.client.get(cmd.Context(), route, false, a.cache)
		if err != nil {
			return err
		}
		return a.print(body, csv)
	}}
	for _, name := range flags {
		values[name] = cmd.Flags().String(name, "", "API "+name+" selector")
	}
	if holdings {
		cmd.Flags().BoolVar(&csv, "csv", false, "Download server-authorized paid holdings CSV (never cached)")
	}
	parent.AddCommand(cmd)
}
func (a *app) print(body []byte, csv bool) error {
	if csv || a.json {
		_, err := a.out.Write(body)
		if err == nil && len(body) > 0 && body[len(body)-1] != '\n' {
			_, err = fmt.Fprintln(a.out)
		}
		return err
	}
	var pretty strings.Builder
	var raw any
	dec := json.NewDecoder(strings.NewReader(string(body)))
	dec.UseNumber()
	if err := dec.Decode(&raw); err != nil {
		return err
	}
	v, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return err
	}
	pretty.Write(v)
	_, err = fmt.Fprintln(a.out, pretty.String())
	return err
}
