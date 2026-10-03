# Usage

## Dashboard

`hubtop` opens the repo list. Each row shows:

- **status dot** — latest workflow run across the repo's workflows:
  `✓` passing, `✗` failing, `●` running, `○` queued, `–` no runs yet
- **name**, **★ stars**, **! open issues/PRs**, **language**, **last push**

Private repos show a 🔒.

Press `enter` for the detail view:

- **Workflow runs** — the 10 most recent runs with branch, short SHA,
  conclusion, duration, and age
- **Latest release** — tag and publish date
- **Open pull requests / issues** — newest 5 each

`esc` goes back, `r` refreshes, `q` quits. Data refreshes automatically
every 60 seconds.

## Script mode

`hubtop --failures` prints one line per repo whose latest run is failing
(`FAIL`) or still running (`RUN`), and exits non-zero when anything
fails. Handy for cron or a pre-push hook:

```sh
hubtop --failures || echo "CI is red somewhere"
```

## Auth details

Resolution order: `GITHUB_TOKEN` env var, then `gh auth token`.
For GitHub Enterprise or tests, `GITHUB_API_BASE` overrides the API
endpoint (e.g. `https://ghe.example.com/api/v3`).
