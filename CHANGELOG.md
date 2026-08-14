## [Unreleased]

### Changed
- **Breaking:** `generate_diagram` with `format=svg` now returns the SVG markup as a text content block instead of an `image/svg+xml` image block. Claude Desktop (the Anthropic API) accepts only raster formats in image blocks, so the previous shape always failed with "unsupported format"; clients that render SVG image blocks now receive markup text instead.
- **Breaking:** the advertised default for `generate_diagram`'s `format` argument changed from `png` to `svg`, so callers that omit it now get the text/SVG path.
- SVG output is normalized for inline chat rendering before being returned: the root element's background and fixed-size style declarations (including Mermaid's `max-width`) are stripped, the fixed `width` becomes `width="100%"` with a `viewBox` (synthesized from the pixel dimensions when missing or invalid) and `style="height:auto"` (the pixel `height` attribute is kept as an intrinsic-size fallback), and `preserveAspectRatio` is dropped in favor of the proportional default. Only the root tag is rewritten: this covers PlantUML-style root backgrounds; backgrounds painted inside the body (Graphviz's canvas polygon, Mermaid's embedded `<style>` rules) are left as-is.
- `generate_diagram` returns an error instead of inline SVG when the normalized markup exceeds 100 KB, pointing the caller at `get_diagram_url` or `generate_png_diagram_with_custom_dpi`, since text output is billed as model context tokens.
- Upgraded `github.com/mark3labs/mcp-go` from v0.25.0 to v0.58.0 (52 releases), migrating to the typed argument accessors API in place of raw `Arguments` map access.
- Raised the required Go toolchain to 1.25.5, updating `go.mod`, CI (`actions/setup-go`), and the Docker builder image accordingly.

### Fixed
- Tool descriptions for the `format` argument no longer advertise `text`, which the schema enum and validation reject; they now list exactly `svg` and `png`.
- `generate_png_diagram_with_custom_dpi` now honors its declared default DPI of 150 when `dpi` is omitted, rejects a wrong-typed `dpi` instead of silently falling back to the default, and enforces a valid range of 72–300 consistently across the guard, the error message, and the parameter schema.
- Stdio server startup/runtime errors are now logged and cause a non-zero exit, except for a graceful SIGTERM/SIGINT shutdown (which surfaces as `context.Canceled` and now exits cleanly).
- Tool annotations are now declared on all three tools (`generate_diagram`, `get_diagram_url`, `generate_png_diagram_with_custom_dpi`) as read-only, non-destructive, idempotent, open-world; `get_diagram_url` previously advertised a self-contradictory `idempotentHint: false`, and `destructiveHint: false` was dropped from the wire due to `omitempty` on a plain bool.
- Mixed-case `diagramType` and `format` arguments are now normalized to lowercase before being forwarded to Kroki, instead of being validated case-insensitively but forwarded raw.
- Corrected the invalid-format error message to list the actually supported formats: `format is required and must be one of: png, svg`.
- `svgconv.Convert` now propagates PNG/JPEG encoding errors instead of discarding them and returning a truncated or corrupt image as success.

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
