# Security advisory template

Use this template for the title, description, and metadata of a GitHub security advisory.
Write for defenders: say who is exposed, what the impact is, and how to reduce the risk.
Do not include exploit steps, timing, syscall sequences, payloads, or proof-of-concept code.
Put affected and patched versions, severity, and CWE in the dedicated advisory fields instead of the description.

Delete the sections that do not apply.

## Title

Use the form `<subject> <active verb> <concrete consequence>`, in sentence case and on one line.
The subject is what the flaw is in or what triggers it, such as a command or a feature.

- Use the present tense and an active verb, such as allows, exposes, bypasses, removes, or leaks.
- State the concrete consequence, such as "bind mount redirection to host path", not "a security issue".
- The subject can also be the flaw class, as in "Race condition in docker cp allows ...".
- Name an API by its feature, not by its endpoint path. Put the path in the description.
- Write commands as plain text, without backticks, because the title is also shown in places that do not render Markdown.
- Avoid hedging with "may" or "could" when the flaw always applies, and do not use "vulnerability" or "security".
  Hedge only when the consequence depends on a condition that the title cannot state.
  Who is affected belongs under "Conditions for exploitation".
- Do not add a `Moby:` prefix, severity words, version numbers, or the advisory's own CVE and GHSA identifiers.
  Naming the identifier of a third-party flaw is fine when it is the cause.

Good titles:

- Race condition in docker cp allows bind mount redirection to host path
- Firewalld reload removes bridge network isolation
- Container build can leak any path on the host into the container (using Git CVE-2022-39253)
- Malicious DNS responses cause insecure-registry fallback
- Oversized request body bypasses AuthZ plugin

## Description

```markdown
## Summary

One or two sentences: what the flaw allows and the main consequence.
Name the feature or API involved.

Then a few sentences on the cause at a high level.
Say which component is responsible and what it fails to guarantee.
Omit how to trigger the flaw reliably.

## Conditions for exploitation

- Configuration the host or container must have.
- Capabilities the attacker needs.
- Action by an operator or API client that must happen.

### Not affected

Optional supplement to the conditions. Include it only when listing what is excluded explains exposure more clearly than the conditions alone.

- Configurations, states, or API endpoints that are not exposed, each with a short reason.
- Platforms or modes with different exposure, such as rootless mode, and how the impact differs.

## Impact

What an attacker gains, and what they do not.

- State what the attacker controls and what they do not control.
- State whether another party, such as an operator or API client, has to start the operation.
- State what persists after the operation completes.

## Workarounds

- Concrete steps that reduce the risk until a fixed version is installed.
- Note the cost of a workaround when it interrupts a service or breaks a workload.
```

## Metadata

The field names follow the GitHub security advisory API.

```json
{
  "summary": "<title>",
  "cvss_vector_string": "CVSS:4.0/<vector>",
  "cwe_ids": ["CWE-<id>"],
  "vulnerabilities": [
    {
      "package": {"ecosystem": "other", "name": "Docker Engine"},
      "vulnerable_version_range": "<= <X.Y.Z>",
      "patched_versions": "<X.Y.Z>"
    },
    {
      "package": {"ecosystem": "go", "name": "github.com/moby/moby/v2/daemon"},
      "vulnerable_version_range": "<= 2.0.0-beta.<N>",
      "patched_versions": "2.0.0-beta.<N>"
    }
  ]
}
```

- Use the title as `summary`.
- Score severity with a CVSS v4 vector instead of picking a severity level.
- Use the most specific CWE that describes the flaw.
- Add one entry per affected package, and list every patched version.

Save the metadata as `metadata.json` and the description as `description.md`.
Then update the advisory in one request, using fish syntax:

```fish
jq --rawfile description description.md '. + {description: $description}' metadata.json \
    | gh api --method PATCH repos/moby/moby/security-advisories/<GHSA-ID> --input -
```

## Guidelines

- Verify every claim in "Not affected" and "Conditions for exploitation" against the code, including which API endpoints reach the affected path.
- Keep each sentence on its own source line.
- Prefer concrete workarounds over general advice such as "only run trusted images".
- Do not overstate the impact: say what the attacker cannot do as well as what they can.
