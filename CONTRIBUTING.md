# Contributing

Open an issue describing the problem before broad changes. Keep the client independent
of site internals: use only documented HTTP APIs, never direct SEC/provider/database access.
Do not include credentials or recorded third-party datasets in tests; use synthetic fixtures.
Run `go test ./...` and `go vet ./...` with Go 1.26.8, or bounded `sh scripts/check.sh`
with Docker on Linux. New behavior should have offline regression tests.
Contributions are licensed under the repository MIT license.
