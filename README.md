# Kroki-MCP

[![CI](https://github.com/utain/kroki-mcp/actions/workflows/ci.yml/badge.svg)](https://github.com/utain/kroki-mcp/actions/workflows/ci.yml)
[![Go Version](https://img.shields.io/badge/go-1.22-blue)](https://golang.org/dl/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](./LICENSE)
<!-- Uncomment and update if Docker image is published
[![Docker Pulls](https://img.shields.io/docker/pulls/utain/kroki-mcp)](https://hub.docker.com/r/utain/kroki-mcp)
-->

Kroki-MCP is a command-line tool and MCP integration for converting textual diagrams (e.g., PlantUML, Mermaid) into images using a Kroki backend. It supports both local and remote Kroki servers, with flexible configuration and multiple output formats.

## Features

- **Modes:**  
  - **SSE:** Streams results using Server-Sent Events.  
  - **STDIO (default):** Reads diagram code from stdin and outputs to stdout.
- **Output Formats:** Supports `png` (default), `svg`, `jpeg`, and `pdf`.
- **Kroki Server:** Configurable backend host (default: `https://kroki.io`).
- **Extensible:** Easily add support for more diagram types and output formats.
- **MCP Integration:** Exposes diagram conversion as an MCP tool using [github.com/mark3labs/mcp-go](https://github.com/mark3labs/mcp-go).

## Usage

```sh
# Default (SSE mode, PNG, default Kroki host)
kroki-mcp

# Specify output format
kroki-mcp --format svg

# Use STDIO mode
kroki-mcp --mode stdio --format pdf

# Specify a custom Kroki server
kroki-mcp --kroki-host http://localhost:8000
```

## Configuration

| Option         | Description                                 | Type    | Default           |
|----------------|---------------------------------------------|---------|-------------------|
| `--host`, `-h`     | Server host address                         | string  | `localhost`        |
| `--port`, `-p`     | Server port                                 | int     | `5090`             |
| `--mode`, `-m`     | Operation mode (`sse` or `stdio`)           | string  | `stdio`            |
| `--format`, `-f`   | Output format (`png`, `svg`, `jpeg`, `pdf`) | string  | `png`              |
| `--kroki-host`     | Kroki server URL                            | string  | `https://kroki.io` |
| `--log-level`      | Log level (`debug`, `info`, `warn`, `error`)| string  | `info`             |
| `--log-format`     | Log format (`text` or `json`)               | string  | `text`             |

## Project Structure

```
kroki-mcp/
├── cmd/
│   └── kroki-mcp/           # Main CLI and MCP server entry point
├── internal/
│   ├── kroki/               # Kroki client logic (HTTP, formats)
│   ├── config/              # Configuration management (flags, env, files)
│   └── mcp/                 # MCP tool/server integration
├── test/                    # Unit and integration tests
├── Dockerfile               # Docker build file
├── docker-compose.yml       # Docker Compose for dev environment
├── go.mod                   # Go module definition
├── README.md                # Project documentation
└── .gitignore               # Git ignore file
```

## Implementation Steps

1. Scaffold Go project.
2. Implement CLI with default SSE mode, format flags.
3. Implement Kroki client supporting all formats.
4. Implement SSE and STDIO modes.
5. Integrate with mcp-go for MCP tool support.
6. Add error handling and tests.
7. Document usage.

## Example: Running as an MCP Server

To run Kroki-MCP as an MCP server from source:

```sh
go run github.com/utain/kroki-mcp/cmd/kroki-mcp --mode sse --format png --kroki-host https://kroki.io
```

You can configure the MCP server in your MCP configuration file as follows:

```json
{
  "mcpServers": {
    "kroki-mcp": {
      "command": "go",
      "args": [
        "run",
        "github.com/utain/kroki-mcp/cmd/kroki-mcp",
        "-m", "stdio",
        "-f", "png",
        "--kroki-host", "https://kroki.io"
      ]
    }
  }
}
```

## Development with Docker

You can use Docker and docker-compose for local development and testing.

### Build and Run with Docker

```sh
docker build -t kroki-mcp .
docker run --rm -it kroki-mcp --help
```

### Using docker-compose (with local Kroki server)

```sh
docker-compose up --build
```

- This will start both a Kroki server and the kroki-mcp service.
- kroki-mcp will connect to the Kroki server at `http://kroki:8000`.

## Contributing

Please see [CONTRIBUTING.md](./CONTRIBUTING.md) for guidelines on how to contribute to this project.

## Changelog

See [CHANGELOG.md](./CHANGELOG.md) for release history.

## Related Projects

- [Kroki](https://kroki.io/)
- [PlantUML](https://plantuml.com/)
- [Mermaid](https://mermaid-js.github.io/)
- [Model Context Protocol (MCP)](https://github.com/modelcontextprotocol)
- [mcp-go](https://github.com/mark3labs/mcp-go)

## License

This project is licensed under the [MIT License](./LICENSE).
