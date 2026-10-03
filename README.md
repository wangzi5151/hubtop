# hubtop

**htop for your GitHub.** A terminal dashboard showing all your repositories'
CI status, open issues/PRs, and latest releases — in one screen, no browser.

```
 hubtop — 3 repos  j/k move · enter detail · r refresh · q quit
 ───────────────────────────────────────────────────────────────
 ● mcpscope           ★12  !3  Go      14m ago
 ● nethole-tester     ★5   !0  Go      1d ago
 ● secret-project 🔒  ★0   !1  Python  32d ago
```

Press `enter` on a repo for workflow runs, the latest release, and open
issues/PRs. The colored dot is the latest CI result: green = passing,
red = failing, yellow = running.

## Install

One-liner (Linux/macOS):

```sh
curl -fsSL https://raw.githubusercontent.com/wangzi5151/hubtop/main/install.sh | sh
```

Or with Go:

```sh
go install github.com/wangzi5151/hubtop/cmd/hubtop@latest
```

Or download a binary from [Releases](https://github.com/wangzi5151/hubtop/releases).

## Auth

hubtop needs a GitHub token. Either:

```sh
export GITHUB_TOKEN=ghp_...        # classic or fine-grained PAT
```

or just `gh auth login` — hubtop falls back to the `gh` CLI's token.

A fine-grained PAT needs **Actions: read**, **Contents: read**,
**Issues: read**, **Pull requests: read**, and **Metadata: read**.
Re-running workflows (`R`) additionally needs **Actions: write**.

## Usage

```sh
hubtop              # open the dashboard
hubtop --failures   # script-friendly CI check: lists failing/running repos,
                    # exits non-zero when something is red
hubtop --version
```

Keybindings:

| Key | Action |
|-----|--------|
| `j`/`k`, `↑`/`↓` | move |
| `enter` | open repo detail |
| `esc` | back to repo list / exit filter |
| `/` | filter repos by name |
| `o` | open repo in browser |
| `R` | re-run the latest failed workflow (detail view) |
| `r` | refresh |
| `q` | quit |

The dashboard auto-refreshes every 60 seconds.

See [docs/USAGE.md](docs/USAGE.md) for details.

## Roadmap

- [x] Re-run failed workflows from the TUI
- [x] Show failed job/step for red runs
- [ ] Filter by CI status (`--only-failing`)
- [ ] Notifications on status change

## License

MIT — see [LICENSE](LICENSE).
