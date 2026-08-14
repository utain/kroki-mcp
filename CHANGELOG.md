## [Unreleased]

### Changed
- Upgraded `github.com/mark3labs/mcp-go` from v0.25.0 to v0.58.0 (52 releases), migrating to the typed argument accessors API in place of raw `Arguments` map access.
- Raised the required Go toolchain to 1.25.5, updating `go.mod`, CI (`actions/setup-go`), and the Docker builder image accordingly.

### Fixed
- `generate_png_diagram_with_custom_dpi` now honors its declared default DPI of 150 when `dpi` is omitted, instead of erroring.
- `generate_png_diagram_with_custom_dpi` now rejects a malformed `dpi` argument instead of silently rendering at the default, and enforces a lower bound of 72 — below roughly that value the rasterizer panicked on small diagrams. The JSON schema, the validation guard and the error message now share a single `[72, 300]` range.
- Stdio server startup/runtime errors are now logged and cause a non-zero exit instead of being silently dropped. A signal-driven shutdown (SIGINT/SIGTERM) surfaces as `context.Canceled` and is still treated as a clean exit.
- `get_diagram_url` tool annotations now correctly advertise the tool as non-destructive; `destructiveHint: false` was previously dropped from the wire due to `omitempty` on a plain bool.

## [v2.0.0] - 2025-05-05

### Added
- Added recommended DPI list and tools for generating PNG diagrams with custom DPI.
- Introduced a new resource for recommended DPI values for diagram generation.
- Registered the recommended DPI list in the MCP server.
- Added a new tool to generate high-quality PNG diagrams with customizable DPI settings.
- Implemented SVG minification and conversion to PNG using specified DPI.
- Updated output formats to include only PNG and SVG, removing TXT and UTXT.
- Created a new SVG conversion package to handle SVG to PNG/JPEG transformations.

# Changelog

## [v1.0.0] - 2025-05-03

### Added
- Initial public release of Kroki-MCP.
- CLI tool for converting textual diagrams (PlantUML, Mermaid) to images via Kroki backend.
- Supports SSE and STDIO modes.
- Output formats: PNG, SVG, JPEG, PDF.
- Configurable backend host, output format, and logging.
- Docker and MCP integration.
- GitHub Actions CI/CD with build, test, SAST, and Docker image publishing.
- Documentation and contribution guidelines.
