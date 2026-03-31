# Contributing to ContainerScope

First off, thank you for considering contributing to ContainerScope! It's people like you that make ContainerScope such a great tool.

## Code of Conduct

This project and everyone participating in it is governed by our [Code of Conduct](CODE_OF_CONDUCT.md). By participating, you are expected to uphold this code.

## How Can I Contribute?

### Reporting Bugs

Before creating bug reports, please check existing issues as you might find out that you don't need to create one. When you are creating a bug report, please include as many details as possible:

- **Use a clear and descriptive title** for the issue
- **Describe the exact steps which reproduce the problem**
- **Provide specific examples to demonstrate the steps**
- **Describe the behavior you observed after following the steps**
- **Explain which behavior you expected to see instead and why**
- **Include screenshots or logs** if applicable
- **Include your environment details** (OS, Docker version, Go version if building from source)

### Suggesting Enhancements

Enhancement suggestions are tracked as GitHub issues. When creating an enhancement suggestion, please include:

- **Use a clear and descriptive title** for the issue
- **Provide a step-by-step description of the suggested enhancement**
- **Provide specific examples to demonstrate the feature**
- **Describe the current behavior** and **explain which behavior you expected to see instead**
- **Explain why this enhancement would be useful**

### Pull Requests

1. **Fork the repo** and create your branch from `master`
2. **If you've added code that should be tested**, add tests
3. **Ensure the code lints** without errors
4. **Update the documentation** if you're changing behavior
5. **Issue that pull request!**

## Development Setup

### Prerequisites

- Go 1.26 or later
- Docker (for testing)
- Access to Docker socket

### Building

```bash
# Clone your fork
git clone https://github.com/PythonicVarun/containerscope.git
cd containerscope

# Build the application
go build -o containerscope ./cmd/containerscope

# Run locally
./containerscope
```

### Building with Docker

```bash
docker build -t containerscope:dev .
docker run -p 4000:4000 -v /var/run/docker.sock:/var/run/docker.sock:ro containerscope:dev
```

## Style Guidelines

### Go Code

- Follow standard Go conventions and idioms
- Use `go fmt` to format your code
- Run `go vet` before submitting
- Keep functions focused and small
- Add comments for exported functions and packages
- Handle errors explicitly

### JavaScript/CSS

- Keep the UI lightweight and performant
- Maintain the existing dark theme aesthetic
- Ensure responsive design
- Test in major browsers (Chrome, Firefox, Safari, Edge)

### Commit Messages

- Use the present tense ("Add feature" not "Added feature")
- Use the imperative mood ("Move cursor to..." not "Moves cursor to...")
- Limit the first line to 72 characters or less
- Reference issues and pull requests liberally after the first line

## Project Structure

```
containerscope/
├── cmd/containerscope/    # Main application entry point
├── internal/
│   ├── app/               # HTTP server, routes, and handlers
│   ├── dockerapi/         # Docker API client wrapper
│   └── ws/                # WebSocket handling for log streaming
├── public/                # Static web assets (HTML, CSS, JS)
├── Dockerfile             # Container build definition
└── docker-compose*.yml    # Compose configurations
```

## Questions?

Feel free to open an issue with your question or reach out to the maintainers.

Thank you for contributing! 🎉
