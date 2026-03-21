# Changelog

All notable changes to this project will be documented in this file.

## [1.0.1] - 2026-03-21

### Added
- --version flag for CLI binary verification

### Changed
- Replace emoji log prefixes with plain text tags ([INFO], [WARN], [OK])
- Compile domain validation regex once at package level for better performance

### Fixed
- Use godotenv.Overload() to enable hot-reload of existing environment variables
- Return 404 for unknown paths instead of catch-all root handler
- Handle case-insensitive protocol prefixes in cleanDomain
- Handle IPv6 addresses correctly in DNS server port detection

## [1.0.0] - 2026-03-21

### Added
- BTK DNS query API endpoint for checking blocked domains
- Build scripts for Windows and Linux
- Godotenv dependency for .env file loading
- Linux systemd service installation guide
- CI/CD pipeline with GitHub Actions and GoReleaser
- Graceful shutdown and HTTP timeouts
- RFC 1035 compliant domain validation
- Panic recovery for watchConfigFile goroutine
- Hot-reload configuration from .env file

### Changed
- Simplified API response structure
- Adapted build files and CI/CD pipeline to project
- Updated build references in README and systemd files
- Updated README to professional emoji-free format

### Fixed
- Return HTTP 400 status code on error responses
- Return JSON parse error to user on POST body failure
- Remove os.Clearenv() to preserve system environment variables
- Fix indirect dependency markers with go mod tidy
- Add automatic Go path detection to build.bat
