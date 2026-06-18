# Review CI and action bumps carefully

For GitHub Actions, CI, toolchain, base image, or packaging bump PRs, review the effective change, not just the version or SHA update.

Check for impact on:

- workflow permissions and token usage
- defaults, inputs, deprecated behavior, and runner compatibility
- test reliability and coverage
- release, packaging, provenance, and published artifacts
- cache behavior, artifact handling, reproducibility, and multi-platform builds

Do not approve only because the change is in workflows, is SHA-pinned, or CI passes.

In the review summary, state what changed, what was inspected, and any remaining risk or recommended verification.
