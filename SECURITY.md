# Security Policy

## Supported Versions

| Version | Supported |
| ------- | --------- |
| latest  | Yes       |

## Reporting a Vulnerability

**Do not open a public GitHub issue for security vulnerabilities.**

Report vulnerabilities by emailing **security@kilnx.org** with:

- Description of the vulnerability
- Steps to reproduce
- Potential impact
- Any suggested fix (optional)

You will receive a response within **48 hours**. If the issue is confirmed, a patch will be released as soon as possible depending on severity.

## Scope

Security issues in scope:

- SQL injection via Kilnx query or model definitions
- Authentication bypass (auth block, session handling, HMAC)
- CSRF protection gaps
- Path traversal in file upload handling
- Remote code execution via compiled binaries
- Privilege escalation via permissions block

Out of scope:

- Vulnerabilities in apps built with Kilnx (report to the app's maintainer)
- Issues in dependencies (report upstream)
- Denial of service via resource exhaustion

## Disclosure Policy

Once a fix is released, the vulnerability will be publicly disclosed in the [CHANGELOG](CHANGELOG.md) with credit to the reporter (unless anonymity is requested).
