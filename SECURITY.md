# Security Policy

## Reporting a Vulnerability

If you discover a security vulnerability, please report it responsibly:

1. **Do not** open a public GitHub issue
2. Use GitHub's **"Report a vulnerability"** button on the Security tab
3. Provide:
   - Description of the vulnerability
   - Steps to reproduce
   - Potential impact assessment
4. Allow reasonable time for a response before disclosing publicly

## Scope

glyph-shift is a file operation tool for AI agents. Security-relevant areas include:

- **Path traversal**: Operations are sandboxed to the workspace root; paths are canonicalized and validated
- **Input validation**: All agent-supplied input is treated as untrusted
- **Binary detection**: Binary files are rejected to prevent unintended processing
- **Race conditions**: The tool does not harden against symlink swaps between validation and open (documented limitation)

## Out of Scope

- Encoding or character set vulnerabilities (encoding is preserved, not interpreted)
- Network-related issues (the tool operates on local filesystem only)
- Vulnerabilities in the AI agent harnesses (Cursor, Claude Code)

## Supported Versions

Security updates are provided for the latest release only.
