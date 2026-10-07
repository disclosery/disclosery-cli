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
