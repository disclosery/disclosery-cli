# Disclosery CLI

Query SEC institutional ownership disclosures through the [Disclosery](https://disclosery.com)
JSON API. This standalone Go client never contacts SEC or other providers directly.

[Installation guide](https://disclosery.com/docs#cli) · [Downloads](https://github.com/disclosery/disclosery-cli/releases/tag/v0.1.0)
· [API contract](https://disclosery.com/llms-api.txt) · [Coverage](https://disclosery.com/data)
· [Examples](https://github.com/disclosery/disclosery-examples)

## Install

Download the archive matching your operating system and architecture from GitHub Releases,
and verify its SHA-256 digest against `checksums.txt`. Extract `disclosery` (Windows:
`disclosery.exe`) into a directory on your PATH. Archives contain LICENSE and dependency
notices. No account or API key is required to download or make a keyless request.

| Operating system | Architectures | Release status |
|---|---|---|
| Linux | amd64, arm64 | amd64 native acceptance; arm64 experimental until native acceptance |
| macOS | amd64 (Intel), arm64 (Apple Silicon) | Unsigned; experimental until native installation acceptance |
| Windows | amd64 (x64), arm64 | Unsigned; experimental until native installation acceptance |

Code signing and notarization are not available for this release. Checksums establish
agreement with the published digest; they are not a signature. Homebrew is planned after
native acceptance; no tap installation command is advertised yet.

```sh
disclosery --version
disclosery --accept-terms --json fund filings 1350694
```

For Go users, the entry package is `github.com/disclosery/disclosery-cli/cmd/disclosery`.
Use the versioned source and release archives until published `go install` acceptance is recorded.

## Configuration

The API defaults to `https://disclosery.com`. Endpoint precedence is `--endpoint` >
`DISCLOSERY_ENDPOINT` > config endpoint > production default. Optional `DISCLOSERY_API_KEY`
overrides the config key, including an explicitly empty value for keyless access.
Keys are sent only in Authorization Bearer headers. Redirects are refused. HTTPS is
required except loopback development HTTP.

Default config is `~/.config/disclosery/config.json` (override with `--config`):

```json
{"endpoint":"https://disclosery.com","api_key":"YOUR_KEY"}
```

Use owner-only permissions for key-bearing config. Never commit it or attach it to an issue.
Keyless access normally allows 20 admitted successful requests per IP per UTC day;
free keys allow 100 and paid keys 2,000. Shared NAT users share keyless allowance.
The server determines current quotas, history and CSV entitlement. Errors exit 1;
success exits 0. Retry-After is reported; quota failures are not automatically retried.

## Commands

```sh
disclosery search 'Bridgewater'
disclosery fund filings 1350694 --limit 5
disclosery fund portfolio 1350694 --quarter 2026q2 --page 1
disclosery fund timeline 1350694 AAPL
disclosery group GROUP_SLUG
disclosery stock holders AAPL --quarter 2026q2
disclosery stock events AAPL --limit 5
disclosery consensus buys
disclosery filing SEC_ACCESSION
```

These are API selectors, not promises of coverage for every identifier or quarter.
Read [coverage](https://disclosery.com/data) and [methods](https://disclosery.com/methods).
Holdings are reported historical disclosures, not current positions or personal assets.

`--json` preserves the original response and decimal precision; default output is
indented JSON. Quota metadata remains in the API envelope; error diagnostics go to stderr. First use displays a personal-use notice;
interactive terminals can acknowledge with `y`, automation uses `--accept-terms`.
`disclosery terms` prints the notice without a request. Acknowledgement is saved beside config.

`fund portfolio --csv` streams the server-authorized paid export directly, never caches
or converts it locally, and is mutually exclusive with `--json`. If export exits with
an error, discard partial output. Server authorization remains in force. The owner elected
to retain CSV/CUSIP for launch and review the recorded rights item separately.
The MIT software license does not grant rights to service data.

## Watch and cache

```sh
disclosery --accept-terms --json watch --once
disclosery --accept-terms --json watch --cursor-file ~/.cache/disclosery/latest.cursor --match Bridgewater
```

Watch uses `/api/v1/latest`; `--scope`, `--key` and `--form` require paid access.
Local `--match` on the unfiltered last-seven-days feed does not establish stock membership.
The default 75-minute interval fits 20 keyless requests/day only when each poll needs
one page. Every page costs an allowance. Checkpoints advance only after complete batches;
failed batches may replay entries, so deduplicate by `filing_id`. Ctrl-C cancels requests/sleep.

Opt in to cache with `--cache --cache-ttl 5m` (maximum 24h). Endpoint/key/query-specific
files are mode 0600 under `~/.cache/disclosery` mode 0700. Cached quotas are historical;
uncached calls are required to verify current authorization. CSV/watch always use fresh requests.

## Upgrade, uninstall and troubleshooting

Download and verify a new version, then replace the old executable. Uninstall by removing
the executable; optionally remove `~/.config/disclosery` and `~/.cache/disclosery`.
Keep configuration if you intend to reinstall. PATH errors mean the installation directory
is not in PATH; wrong-architecture errors require a different archive. Use `--accept-terms`
for pipes. A 401 requires a valid key or keyless access; 403 may indicate plan restrictions;
429 requires waiting until Retry-After/reset. Empty coverage requires an available reporter/period.

[Report a bug](https://github.com/disclosery/disclosery-cli/issues); follow
[SECURITY.md](SECURITY.md) for private credential/security reports. Carlos maintains releases.

## Development

Go 1.26.8. `go test ./...` and `go vet ./...` are offline and use synthetic httptest
responses. `sh scripts/check.sh` performs capped Docker checks (2 CPUs, 2 GiB, two workers).
`LOCAL_RELEASE=1 sh scripts/check.sh` creates six version-stamped candidates and checksums.
`RELEASE_VERSION=0.1.0` selects the version. Publication is disabled in GoReleaser;
owner-reviewed tag releases use the controlled manual GitHub Actions workflow.

Licensed under [MIT](LICENSE). Dependency licenses are reproduced in
[THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md). The private site and its data are not included.
