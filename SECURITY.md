# Security policy

mcpDeck sits between AI agents and real systems and holds credentials for
them, so security reports get priority over everything else.

## Reporting a vulnerability

Please **don't open a public issue**. Use GitHub's private reporting:
[Report a vulnerability](https://github.com/abhijeet-oxide/mcpDeck/security/advisories/new).

Include what you did, what you expected and what happened, and the version
(`mcpdeck version`). You'll get an acknowledgement within 3 working days. Fixes
ship as a patch release with an advisory that credits you, unless you'd
rather not be named.

## In scope

Anything that breaks one of the guarantees in [AGENTS.md](AGENTS.md#invariants-never-break-these), for example:

- a credential reaching an agent, a response, a log, a recorded call or an error;
- an upstream request that bypasses the proxy's allow-list or grants;
- a read grant that can write;
- a connection token that can change access, approve, or read secrets;
- a web page or another machine reaching the local server;
- tool code escaping the JavaScript sandbox.

## Supported versions

The latest release. mcpDeck releases on every change to `main`, so a fix is
available as soon as it merges.

## Verifying a release

Every file in a release is listed in `checksums.txt` and has a build
provenance attestation that ties it to the workflow run that built it:

```sh
gh attestation verify mcpdeck_linux_amd64.tar.gz --repo abhijeet-oxide/mcpDeck
```

Each archive also has an SBOM (`*.sbom.json`).
