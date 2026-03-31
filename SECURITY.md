# Security Policy

## Supported Versions

| Version | Supported          |
| ------- | ------------------ |
| latest  | :white_check_mark: |

## Reporting a Vulnerability

We take security seriously. If you discover a security vulnerability within ContainerScope, please follow these steps:

### Do NOT

- Open a public GitHub issue for security vulnerabilities
- Disclose the vulnerability publicly before it has been addressed

### Do

1. **Email the maintainer** at hello@pythonicvarun.me with details of the vulnerability
2. **Include the following information:**
   - Type of vulnerability (e.g., XSS, injection, privilege escalation)
   - Full paths of source file(s) related to the vulnerability
   - Step-by-step instructions to reproduce the issue
   - Proof-of-concept or exploit code (if possible)
   - Impact of the issue, including how an attacker might exploit it

### What to Expect

- **Acknowledgment**: We will acknowledge receipt of your vulnerability report within 48 hours
- **Communication**: We will keep you informed about the progress toward a fix
- **Credit**: If you would like, we will credit you in the release notes when the vulnerability is fixed

### Rewards

This is a personal open-source project and does not offer monetary rewards or bug bounties. However, we deeply appreciate security researchers who help keep ContainerScope secure and will gladly provide public acknowledgment and credit for valid reports.

## Security Considerations

### Docker Socket Access

ContainerScope requires access to the Docker socket (`/var/run/docker.sock`) to function. This is a privileged resource that provides control over the Docker daemon.

**Recommendations:**

1. **Use read-only access**: Always mount the Docker socket as read-only (`:ro`)
   ```bash
   -v /var/run/docker.sock:/var/run/docker.sock:ro
   ```

2. **Network isolation**: Run ContainerScope in an isolated network if possible

3. **Access control**: Limit who can access the ContainerScope web interface
   - Use a reverse proxy with authentication
   - Bind to localhost only in sensitive environments

4. **Trusted environments**: Only deploy ContainerScope in environments where you trust all users who can access it

### Web Interface

- The web interface does not implement authentication by default
- Deploy behind a reverse proxy with authentication for production use
- Use HTTPS in production environments

## Best Practices

1. **Keep updated**: Always run the latest version of ContainerScope
2. **Minimal privileges**: Run with the least privileges necessary
3. **Network security**: Use firewalls and network policies to restrict access
4. **Audit logs**: Monitor access to the ContainerScope service
