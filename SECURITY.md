# Security policy

## Supported versions

Security fixes are provided for the latest published release. Older versions
may be asked to update before a report is investigated.

## Reporting a vulnerability

Please use GitHub's
[private vulnerability reporting](https://github.com/LickABrick/Inpakker/security/advisories/new)
instead of opening a public issue.

Include the affected version, expected and observed behavior, reproduction
steps, and potential impact. Remove credentials, tenant information,
proprietary application content, and unrelated personal data. You should receive
an acknowledgement within seven days.

Please do not publicly disclose a vulnerability until a fix or coordinated
disclosure plan is available.

## Release integrity

Official builds are published only through this repository's GitHub Releases.
Release ZIPs have SHA-256 checksums, the checksum manifest has a detached ECDSA
signature, and public builds receive GitHub provenance attestations. Inpakker's
self-update command requires both the trusted signature and matching checksum.

Microsoft's packaging utility and the optional community decoder are external
programs. Obtain and assess them from their respective upstream projects.
