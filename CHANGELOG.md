# Changelog

All notable changes documented here. Format: [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased] - 2026-08-13
### Changed
- docs: comprehensive README + topics
- fix(ci): pass github_token to buf-setup-action to avoid rate-limit download failures

## [Unreleased] - 2026-08-12
### Changed
- fix: rate limiter fixed window; correct stats field; add cache tests
- ci: add auto-issue-triage, release-notes, changelog via chirag127/workflows
- fix: reject URLs with empty host in validateURL
- fix: resolve all golangci-lint issues
- fix: update golangci-lint to v2.12.2 and align go.mod version
- fix: suppress PACKAGE_DIRECTORY_MATCH buf lint rule
- fix: remove googleapis dep and http annotations from proto
- feat: initial implementation of go-vault URL shortener microservice
