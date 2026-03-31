# ContainerScope

A lightweight, real-time Docker container log viewer with a modern web interface.

![Go Version](https://img.shields.io/badge/Go-1.26-00ADD8?logo=go&logoColor=white)
![License](https://img.shields.io/badge/License-MIT-blue.svg)
![Docker](https://img.shields.io/badge/Docker-Supported-2496ED?logo=docker&logoColor=white)

## Features

- 🔴 **Real-time log streaming** via WebSocket
- 🐳 **Auto-discovers** all running Docker containers
- 🔍 **Filter logs** with instant search
- 📜 **Separate stdout/stderr** viewing
- 🎯 **Follow mode** to auto-scroll to latest logs
- 🖥️ **Clean, modern UI** with dark theme
- ⚡ **Lightweight** - single binary, minimal footprint
- 🔒 **Read-only** access to Docker socket

## Quick Start

### Using Docker (Recommended)

```bash
docker run -d \
  --name containerscope \
  -p 4000:4000 \
  -v /var/run/docker.sock:/var/run/docker.sock:ro \
  containerscope:latest
```

Then open [http://localhost:4000](http://localhost:4000) in your browser.

### Using Docker Compose

```yaml
services:
    containerscope:
        image: containerscope:latest
        container_name: containerscope
        restart: unless-stopped
        ports:
            - "4000:4000"
        volumes:
            - /var/run/docker.sock:/var/run/docker.sock:ro
        environment:
            - CONTAINER_SCOPE_PORT=4000
```

```bash
docker compose up -d
```

### Building from Source

Requirements:

- Go 1.26 or later

```bash
# Clone the repository
git clone https://github.com/PythonicVarun/containerscope.git
cd containerscope

# Build the binary
go build -o containerscope ./cmd/containerscope

# Run (requires access to Docker socket)
./containerscope
```

## Configuration

| Environment Variable   | Default | Description      |
| ---------------------- | ------- | ---------------- |
| `CONTAINER_SCOPE_PORT` | `4000`  | HTTP server port |

## Architecture

```
containerscope/
├── cmd/containerscope/    # Application entry point
├── internal/
│   ├── app/               # HTTP server and handlers
│   ├── dockerapi/         # Docker API client
│   └── ws/                # WebSocket handling
└── public/                # Static web UI assets
```

## Usage

1. **Start ContainerScope** using one of the methods above
2. **Open the web UI** at `http://localhost:4000`
3. **Select a container** from the sidebar to view its logs
4. **Use the toolbar** to:
    - Filter logs with the search bar
    - Toggle stderr visibility
    - Enable/disable follow mode
    - Clear the log buffer

## Security Considerations

ContainerScope requires read-only access to the Docker socket (`/var/run/docker.sock`). This is necessary to:

- List running containers
- Stream container logs

**Important:** The Docker socket provides access to the Docker daemon. Only run ContainerScope in trusted environments.

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request. For major changes, please open an issue first to discuss what you would like to change.

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'feat: add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

Please make sure to update tests as appropriate.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Acknowledgments

- Built with [Go](https://golang.org/)
- Docker API integration
- Modern CSS with dark theme
