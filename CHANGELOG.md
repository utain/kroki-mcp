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
