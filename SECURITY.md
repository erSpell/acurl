# Security Policy

acurl is maintained by [tragent.io](https://tragent.io).

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

`acurl tap` runs a local proxy that sees your AI CLI traffic in clear text:

- It only listens on loopback and refuses requests with an `Origin` header or a non-loopback
  `Host`, so web pages cannot reach it. Any program running as you on the same machine can.
- Credentials are redacted by default. Prompts, code and model output are not: protect
  `-w` session files like shell history, and do not attach them to public issues.
- `--keep-credentials` keeps API keys and tokens in captures. Use it only for local debugging.
