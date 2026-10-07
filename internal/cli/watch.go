package cli

import (
	"encoding/json"
	"fmt"
	"github.com/spf13/cobra"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type latest struct {
	Data struct {
		Cursor  int64             `json:"cursor"`
		Next    int64             `json:"next_cursor"`
		Filings []json.RawMessage `json:"filings"`
	} `json:"data"`
}

func (a *app) watch() *cobra.Command {
	var after int64
	var interval time.Duration
	var match, scope, key, form, cursorFile string
	var once bool
	cmd := &cobra.Command{Use: "watch", Short: "Poll /api/v1/latest with a filing-ID cursor; exit successfully on a text match", Long: "Poll stored filings; each page consumes an API call. Keyless access allows 20 calls per day. --match searches returned filing JSON locally. Server scope/key/form filters require paid access. --once fetches one complete batch. --json writes one filing JSON per line.", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		if after < 0 || after > 2147483647 {
			return fmt.Errorf("--after must be a nonnegative int32 filing ID")
		}
		if interval < time.Second {
			return fmt.Errorf("--interval must be at least 1s")
		}
		if a.cache {
			return fmt.Errorf("watch cannot use --cache")
		}
		if cursorFile != "" {
			b, e := os.ReadFile(cursorFile)
			if e == nil {
				v, e := strconv.ParseInt(strings.TrimSpace(string(b)), 10, 32)
				if e != nil || v < 0 {
					return fmt.Errorf("invalid cursor file")
				}
				if cmd.Flags().Changed("after") {
					return fmt.Errorf("use either an existing cursor file or --after")
				}
				after = v
			} else if !os.IsNotExist(e) {
				return e
			}
		}
		for {
			original := after
			before := int64(0)
			high := after
			matched := false
			for {
				q := url.Values{"limit": {"100"}}
				if original > 0 {
					q.Set("after", strconv.FormatInt(original, 10))
				}
				if before > 0 {
					q.Set("before", strconv.FormatInt(before, 10))
				}
				for k, v := range map[string]string{"scope": scope, "key": key, "form": form} {
					if v != "" {
						q.Set(k, v)
					}
				}
				body, e := a.client.get(cmd.Context(), "/api/v1/latest?"+q.Encode(), false, false)
				if e != nil {
					return e
				}
				var batch latest
				if e = json.Unmarshal(body, &batch); e != nil {
					return e
				}
				if batch.Data.Cursor < 0 || batch.Data.Cursor > 2147483647 {
					return fmt.Errorf("API returned invalid latest cursor")
				}
				if len(batch.Data.Filings) > 0 && batch.Data.Cursor <= original {
					return fmt.Errorf("API returned latest cursor before saved cursor")
				}
				if batch.Data.Cursor > high {
					high = batch.Data.Cursor
				}
				for _, row := range batch.Data.Filings {
					if match != "" && !strings.Contains(strings.ToLower(string(row)), strings.ToLower(match)) {
						continue
					}
					matched = true
					if e = a.print(row, false); e != nil {
						return e
					}
				}
				next := batch.Data.Next
				if next == 0 {
					break
				}
				if len(batch.Data.Filings) == 0 || next <= original || (before > 0 && next >= before) {
					return fmt.Errorf("API returned non-progressing latest cursor")
				}
				before = next
			}
			// Only checkpoint after every page succeeds. A failed batch may replay, never skip.
			after = high
			if cursorFile != "" {
				if e := atomicWrite(cursorFile, []byte(strconv.FormatInt(after, 10)+"\n")); e != nil {
					return e
				}
			}
			if match != "" && matched {
				return nil
			}
			if once {
				return nil
			}
			timer := time.NewTimer(interval)
			select {
			case <-cmd.Context().Done():
				timer.Stop()
				return cmd.Context().Err()
			case <-timer.C:
			}
		}
	}}
	cmd.Flags().Int64Var(&after, "after", 0, "Only filings with IDs greater than this cursor")
	cmd.Flags().StringVar(&cursorFile, "cursor-file", "", "Persist last completely processed latest_filing_id cursor")
	cmd.Flags().DurationVar(&interval, "interval", 75*time.Minute, "Poll spacing (20 keyless calls/day before pagination)")
	cmd.Flags().StringVar(&match, "match", "", "Case-insensitive text in filing JSON; exit 0 after the complete matching batch")
	cmd.Flags().StringVar(&scope, "scope", "", "Paid server filter: funds, groups, insiders or stocks")
	cmd.Flags().StringVar(&key, "key", "", "Paid scope lookup key")
	cmd.Flags().StringVar(&form, "form", "", "Paid SEC form filter")
	cmd.Flags().BoolVar(&once, "once", false, "Process one complete batch then exit")
	return cmd
}
