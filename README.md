# repojack

[![CI](https://github.com/ucsurf/repojack/actions/workflows/ci.yml/badge.svg)](https://github.com/ucsurf/repojack/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/ucsurf/repojack.svg)](https://pkg.go.dev/github.com/ucsurf/repojack)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

**Find dependencies that point at a GitHub account that no longer exists, so anyone can register it and serve you their code.**

When a GitHub user or org is renamed or deleted, its old name becomes free. Whoever registers it next owns `github.com/<old-name>/<repo>`: your `go.mod`, your `uses:` lines, your `git+https://` pins and your submodules now fetch from them. This is called *repojacking*. GitHub's "retired namespace" protection only covers repos with more than 100 clones in the week before the rename, and Go modules served from the module proxy are rarely cloned.

`repojack` reads your manifests and asks github.com whether each owner still exists. That's all it does.

![demo](docs/demo.gif)

```sh
go install github.com/ucsurf/repojack/cmd/repojack@latest
```

Or grab a binary from the [releases](https://github.com/ucsurf/repojack/releases).

## Example

```console
$ repojack testdata/proj
testdata/proj/.github/workflows/ci.yml:9  ghost-owner-rj404/setup-thing  [action]
  owner does not exist; the name can be registered by anyone
  > - uses: ghost-owner-rj404/setup-thing/sub@v1

testdata/proj/.gitmodules:3  ghost-owner-rj404/sub  [submodule]
  owner does not exist; the name can be registered by anyone
  > url = git@github.com:ghost-owner-rj404/sub.git

testdata/proj/go.mod:7  ghost-owner-rj404/oldlib  [go]
  owner does not exist; the name can be registered by anyone
  > github.com/ghost-owner-rj404/oldlib v1.2.3 // indirect

testdata/proj/go.mod:10  renamed-owner-rj404/cobra  [go]
  owner does not exist; the name can be registered by anyone
  > replace github.com/spf13/cobra => github.com/renamed-owner-rj404/cobra v1.8.1

testdata/proj/package.json:5  ghost-owner-rj404/thing  [npm]
  owner does not exist; the name can be registered by anyone
  > thing: github:ghost-owner-rj404/thing#v1

testdata/proj/requirements.txt:2  ghost-owner-rj404/pylib  [pip]
  owner does not exist; the name can be registered by anyone
  > git+https://github.com/ghost-owner-rj404/pylib.git@main#egg=pylib

10 GitHub references checked, 6 hijackable
```

And on a real project (Grafana, 3,792 GitHub references across its `go.mod`, `package.json` and workflow files):

```console
$ repojack ~/src/grafana
3792 GitHub references checked, 0 hijackable
```

When the old repository still redirects to its new home, `repojack` tells you where to point instead:

```
  owner renamed or deleted; repository now redirects, the old name can be registered by anyone
  fix: point to github.com/new-owner/lib
```

Exit code is `1` when something is hijackable, `2` on usage errors, `0` otherwise.

## What it reads

| File | References |
|------|------------|
| `go.mod` | `require` and `replace` lines on `github.com/...` (the module's own path is ignored) |
| `.github/workflows/*.yml`, `action.yml` | `uses: owner/repo@ref` (local `./` and `docker://` are ignored) |
| `package.json` | `github:owner/repo`, `owner/repo` shorthand and GitHub URLs in dependencies, overrides and resolutions |
| `requirements*.txt`, `pyproject.toml`, `Pipfile` | `git+https://github.com/...` |
| `Cargo.toml` | `git = "https://github.com/..."` |
| `.gitmodules` | `url = ...github.com...` |

`node_modules`, `vendor`, `testdata`, `.git` and virtualenvs are skipped. `-list` prints every reference it found without touching the network.

## Flags

```
-format text|json|sarif   output format (default text)
-ignore a,b/c             owners or owner/repo to skip
-concurrency N            parallel requests to github.com (default 4)
-list                     only list references, no network
-version
```

## How it decides

For each unique owner it sends one `HEAD https://github.com/<owner>` request. `404` means the account does not exist and the name is free to register. Only then does it check `github.com/<owner>/<repo>`: a redirect means the account was renamed and the dependency still works today but can be taken over at any moment.

It talks to the github.com website, **not the REST API**: no token, no API rate limit, nothing to configure in CI. Results are cached per owner, so a project with thousands of references needs only a few hundred requests.

## How is this different from…

- [ChainJacking](https://github.com/Checkmarx/chainjacking) and [gobelin](https://github.com/boost-rnd/gobelin) check **Go modules only** and need a GitHub API token (or hit the 60-requests/hour anonymous limit).
- [octoscan](https://github.com/synacktiv/octoscan) has a repo-jacking rule for **workflow `uses:` only**, among many other rules.
- `repojack` covers Go, Actions, npm, pip, Cargo and submodules in one single static binary, needs no token, and reports only one thing.

## GitHub Action

```yaml
name: repojack
on:
  push:
  pull_request:
  schedule:
    - cron: "0 6 * * 1"   # owners can disappear at any time, so re-check weekly
permissions:
  contents: read
jobs:
  repojack:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          persist-credentials: false
      - uses: ucsurf/repojack@v0.1.0
```

Inputs: `path` (default `.`), `ignore`, `sarif` (path for a SARIF report), `version` (release to use; defaults to the tag you pinned).
The action downloads the prebuilt release binary for the runner and verifies it against `checksums.txt`; no Go toolchain needed.

## pre-commit

```yaml
repos:
  - repo: https://github.com/ucsurf/repojack
    rev: v0.1.0
    hooks:
      - id: repojack
```

## Limits

- An owner that exists but was **recreated by someone else** looks fine to `repojack`. Pin by commit SHA or checksum (`go.sum`, lockfiles) to catch content changes.
- It cannot tell whether GitHub has retired a popular namespace; treat every finding as real until you move off it.
- Only github.com references are checked.

## License

MIT
