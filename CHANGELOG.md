# Changelog

## v0.1.1

- Fix: `package.json` references came out in random order (Go map iteration), so `-list` and JSON output weren't reproducible. Found by fuzzing.
- Fuzz test for manifest extraction across all supported file kinds, plus edge-case tests.
- README: release and Scorecard badges, Why section, more limitations. OpenSSF Scorecard workflow.

## v0.1.0

- First release: go.mod, GitHub Actions, package.json, requirements/pyproject/Pipfile, Cargo.toml and .gitmodules.
- Text, JSON and SARIF output; GitHub Action (prebuilt binary, checksum verified) and pre-commit hook.
