---
applyTo: "**/*_test.go,integration/**,integration-cli/**"
---

# Review tests

- Prefer integration tests under `integration/` that exercise the daemon through the API client over unit tests that mock out most of the code under test.
  When an existing table-driven integration test covers the same area, suggest adding a case to it instead of a new test.
- New tests must not be added under `integration-cli/`.
- Flag tests that add little value, such as tests longer than the trivial function they cover or tests that only re-check standard library behavior.
- Check that tests cover the edge cases the change introduces, such as an empty value (`[]`, `""`) versus an unset one.
- Test helpers that make assertions should call `t.Helper()`.
