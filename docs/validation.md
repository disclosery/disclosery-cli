# Validation — 2026-10-07

Root orchestrator ran bounded Go checks: 2 CPUs, 2 GiB RAM, no swap, two Go workers.
`go mod tidy`, formatting, `go vet ./...`, and the complete offline suite passed.
There are 12 top-level tests, including four endpoint precedence and nine command subcases.
No API quota or credentials are needed for the offline suite.

Six targets cross-compiled at version 0.1.0. Native Linux amd64 `--version` and `--help`
passed. Production CLI acceptance and final public-download hashes are recorded by the
launch orchestrator in feature 007. macOS/Windows are unsigned; targets lacking native
execution evidence remain experimental. Cross-compilation is not native acceptance.

Provider-derived recorded API fixtures from the private development tree were excluded.
Public tests contain only handcrafted synthetic httptest responses. Dependency licenses
and the Go runtime BSD license are reproduced in THIRD_PARTY_NOTICES.md.

Build flags disable VCS metadata and trim build paths. Archives include the README,
software license and dependency notices. Release automation pins the Go toolchain,
checks the reviewed workflow commit with bounded Docker resources, and creates immutable
version tags/releases only after checks succeed. The owner manually starts publication.

## Published v0.1.0 acceptance

Source commit: `2f944eaa005b882cc03ef9dd5ed4a6b9b221af85`.
[Release workflow 37635056168](https://github.com/disclosery/disclosery-cli/actions/runs/37635056168)
succeeded. All six [public release archives](https://github.com/disclosery/disclosery-cli/releases/tag/v0.1.0)
were downloaded, verified against `checksums.txt`, and inspected for the binary, README,
MIT license and third-party notices. ZIP integrity checks passed.

The downloaded Linux amd64 binary reports `disclosery version 0.1.0`; help and the
[website installation guide](https://disclosery.com/docs#cli) first-command recipe passed:

```sh
disclosery --accept-terms --json fund filings 1350694 --limit 1
```

Production search, filing history, a 2026 Q2 portfolio and a bounded one-shot watch were
also exercised. Anonymous CSV was refused with `403 paid_required`; an invalid synthetic
key was refused with `401 unauthorized`. Authorized CSV success is covered by synthetic
offline tests; a live paid-key export was not tested. CSV/CUSIP remains available under
existing server authorization, with the owner's data-rights review deferred.

Linux amd64 is validated. Linux arm64, both macOS targets and both Windows targets remain
experimental pending native acceptance. No signing or notarization is claimed.
