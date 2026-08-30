# Security Policy

## Supported versions

Security fixes are released for the latest minor version.

## Reporting a vulnerability

Please **do not** open a public issue for security problems.

Report privately through GitHub's
[private vulnerability reporting](https://github.com/tragent-io/acurl/security/advisories/new),
or email **security@tragent.io** with:

- a description of the issue and its impact
- steps to reproduce
- the acurl version (`acurl --version`)

You can expect an acknowledgement within 3 working days and a status update within 7 days.
Reporters are credited in the release notes unless they prefer otherwise.

## Things to keep in mind

acurl makes HTTP requests on your behalf:

- Custom profile files can set arbitrary headers. Only load profile files you trust.
- Requests come from your own IP address.
- Do not run acurl with elevated privileges.
