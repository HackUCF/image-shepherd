# Security Policy

## Supported Versions

We release patches for security vulnerabilities. Which versions are eligible for receiving such patches depends on the CVSS v3.0 Rating:

| Version | Supported          |
| ------- | ------------------ |
| latest  | :white_check_mark: |
| < latest| :x:                |

## Reporting a Vulnerability

The HackUCF team and community take all security bugs in image-shepherd seriously.

### When Should I Report a Vulnerability?

- You think you discovered a potential security vulnerability in image-shepherd
- You are unsure how a vulnerability affects image-shepherd
- You think you discovered a vulnerability in another project that image-shepherd depends on

### When Should I NOT Report a Vulnerability?

- You need help tuning image-shepherd components for security
- You need help applying security related updates
- Your issue is not security related

### How to Report a Vulnerability

Please report security vulnerabilities by emailing the maintainers at:

**security@hackucf.org**

Please include the following information in your report:

- Type of issue (e.g. buffer overflow, SQL injection, cross-site scripting, etc.)
- Full paths of source file(s) related to the manifestation of the issue
- The location of the affected source code (tag/branch/commit or direct URL)
- Any special configuration required to reproduce the issue
- Step-by-step instructions to reproduce the issue
- Proof-of-concept or exploit code (if possible)
- Impact of the issue, including how an attacker might exploit the issue

This information will help us triage your report more quickly.

### Response Timeline

- We will acknowledge receipt of your vulnerability report within 48 hours
- We will provide a detailed response within 7 days indicating the next steps in handling your report
- We will keep you informed of the progress towards a fix and full announcement
- We may ask for additional information or guidance

### Security Update Process

1. **Acknowledgment**: We acknowledge the vulnerability report
2. **Assessment**: We assess the vulnerability and determine its severity
3. **Fix Development**: We develop a fix for the vulnerability
4. **Testing**: We test the fix to ensure it resolves the issue without introducing new problems
5. **Release**: We release a new version with the fix
6. **Disclosure**: We publicly disclose the vulnerability details after the fix is available

### Preferred Languages

We prefer all communications to be in English.

### Comments on this Policy

If you have suggestions on how this process could be improved please submit a pull request.

## Security Best Practices

When using image-shepherd, please follow these security best practices:

### Container Security
- Always use the latest version of the container image
- Run containers with non-root users when possible
- Use read-only file systems when applicable
- Regularly scan your container images for vulnerabilities

### Image Processing
- Validate all input images before processing
- Implement proper access controls for image files
- Use secure temporary directories with appropriate permissions
- Monitor and log image processing activities

### Infrastructure Security
- Keep your host systems and dependencies up to date
- Use proper network segmentation
- Implement monitoring and alerting for security events
- Regular security audits and penetration testing

## Known Security Considerations

### Image File Handling
- Large image files may cause memory exhaustion
- Malformed image files could potentially cause crashes
- Ensure adequate disk space for image processing operations

### Network Operations
- API endpoints should be properly authenticated
- Use HTTPS for all network communications
- Implement rate limiting to prevent abuse

## Third-Party Dependencies

We regularly monitor our dependencies for security vulnerabilities using:
- Dependabot for automated dependency updates
- Security scanning in our CI/CD pipeline
- Regular security audits of critical dependencies

## Security Tools and Scanning

Our CI/CD pipeline includes:
- Static security analysis with Trivy
- Dependency vulnerability scanning
- Container image scanning
- Infrastructure as Code security scanning

For questions about security practices or concerns not covered in this policy, please contact us at security@hackucf.org.