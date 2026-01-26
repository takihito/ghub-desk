# Security Policy

## Supported Versions

The following versions of ghub-desk are currently supported with security updates.

| Version | Supported |
|--------|-----------|
| Latest minor release | ✅ |
| Older versions       | ❌ |

Only the latest released version is supported.  
Users are strongly encouraged to upgrade to the latest version when a new release is published.

---

## Reporting a Vulnerability

If you discover a security vulnerability in ghub-desk, please report it **privately**.

### Preferred Method

- Use **GitHub Security Advisories**
  - Navigate to:  
    https://github.com/takihito/ghub-desk/security/advisories
  - Click **"Report a vulnerability"**

This allows encrypted, private communication and coordinated disclosure.

### Please Include

When reporting, include as much detail as possible:

- A clear description of the vulnerability
- Steps to reproduce (proof-of-concept if available)
- Affected versions
- Potential impact (data exposure, privilege escalation, etc.)

---

## Disclosure Process

1. The maintainer will acknowledge the report as soon as reasonably possible.
2. The issue will be assessed and validated.
3. A fix will be developed and released.
4. A GitHub Security Advisory will be published if appropriate.

Please **do not** open public GitHub issues or discussions for security-related reports.

---

## Scope

### In Scope
- ghub-desk CLI
- ghub-desk MCP server functionality
- Local SQLite handling
- GitHub API interactions
- Authentication and token handling

### Out of Scope
- Misconfiguration of GitHub permissions by users
- Compromised GitHub accounts or tokens outside ghub-desk
- Issues caused by unsupported or modified builds

---

## Dependencies

ghub-desk relies on third-party open source dependencies.  
Known vulnerabilities in dependencies are tracked via GitHub Dependabot and addressed as part of regular maintenance.

---

## Secure Development Practices

- Public source code with peer review
- CI-based automated testing
- Dependency vulnerability monitoring
- Principle of least privilege for GitHub API usage

---

Thank you for helping improve the security of ghub-desk.
