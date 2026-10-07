# Release checklist

- Run bounded scripts/check.sh; review changes independently.
- Confirm source contains no credentials, recorded provider data or local configuration.
- Build version-stamped six-target archives and verify their SHA-256 checksums.
- Exercise production search, filings, portfolio and bounded watch.
- Record native acceptance by platform; mark untested targets experimental.
- Confirm unsigned macOS and Windows status; signing is deferred.
- Retain CSV server authorization. Resolve an export restriction only if concretely established.
- Publish reviewed source, tag, immutable assets, checksums and release notes.
- Confirm public downloads and installation from website links.
- Update website homepage/docs, GitHub profile and examples.
- For corrections publish a new version; never replace a released artifact silently.

Carlos maintains releases. Homebrew and code signing are later channels.
