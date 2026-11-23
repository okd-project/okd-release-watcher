# OKD Release Watcher

A CLI tool and Slack bot backend that monitors OKD release streams for failed and rejected builds. It fetches job failure details from the release controller and enriches reports with AI-powered root cause analysis from the [claude-payload-agent](https://github.com/openshift/release).

## Features

- Monitors configurable OKD release streams for Failed/Rejected payloads
- Reports blocking and informing job failures with Prow links
- Fetches claude-payload-agent analysis summaries and root cause information
- JSON output mode for programmatic consumption
- Slack bot mode for channel reporting

## Installation

```bash
go install github.com/okd-project/okd-release-watcher/cmd/okd-release-watcher@latest
```

Or build from source:

```bash
git clone https://github.com/okd-project/okd-release-watcher.git
cd okd-release-watcher
go build -o okd-release-watcher ./cmd/okd-release-watcher
```

## Usage

### CLI Report

```bash
# Default: check the last 24h of nightly streams
./okd-release-watcher report

# Custom lookback window
./okd-release-watcher report --lookback 48h

# Include healthy (accepted) builds
./okd-release-watcher report --include-healthy

# Custom streams
./okd-release-watcher report --streams 4.22.0-0.okd-scos-nightly,5.0.0-0.okd-scos-nightly

# JSON output (for AI agents or scripts)
./okd-release-watcher report --json
```

### Slack Bot

```bash
export TOKEN=xoxb-your-slack-bot-token
./okd-release-watcher bot
```

The bot responds to messages with:
- `help` — show available commands
- `report` — generate a health report
- `report lookback=48h healthy` — report with custom options

### Docker

```bash
docker build -t okd-release-watcher .
docker run okd-release-watcher report
```

## Default Streams

| Stream | Description |
|--------|-------------|
| `4.22.0-0.okd-scos-nightly` | OKD SCOS 4.22 nightly builds |
| `5.0.0-0.okd-scos-nightly` | OKD SCOS 5.0 nightly builds |

## Report Output

The report shows for each stream:
- Build counts (Accepted/Rejected/Failed) within the lookback window
- Per failed/rejected build:
  - Blocking job failures with Prow URLs
  - Informing job failures with Prow URLs
  - Link to the claude-payload-agent HTML analysis
  - Root cause summaries from the AI analysis
  - Current rejection streak length

## Postsubmit Watcher (`okd-postsubmit-watcher`)

A separate binary that reports on the OKD SCOS postsubmit image jobs
(`branch-ci-<org>-<repo>-<branch>-okd-scos-images`) and on how fresh the images in
the latest accepted OKD CI payloads are. It does not run as part of `okd-release-watcher`.

```bash
go build -o okd-postsubmit-watcher ./cmd/okd-postsubmit-watcher

# Text report: runs in the last 12h + image state for 5.0 and 5.1
./okd-postsubmit-watcher report

# HTML + Slack summary + JSON
./okd-postsubmit-watcher report --window 24h --stale-after 30d \
  --html report.html --slack slack.txt --json > report.json
```

The report has two sections:

1. **Recent runs**: every OKD postsubmit run started in `--window`, from Prow's live job
   list (`prowjobs.js`, which keeps about 24h).
2. **Image state per stream**: every image in the stream's latest accepted payload, with:
   - when the payload's image was built
   - the payload commit
   - the job's latest result and commit, from stored results in Google Cloud Storage
   - flags:
     - **failing**: the latest run failed, so the image is stuck at its previous build
     - **pending**: the job built a newer commit that isn't in the payload yet
     - **stale**: built more than `--stale-after` ago
     - **no-run**: no job results in Google Cloud Storage, which keeps about 60 days

Data sources: Prow, `test-platform-results-public` in Google Cloud Storage, and the
release controller's `/releasetag/<tag>/json`. All of these are public. Image build
times are read from `quay-proxy.ci.openshift.org`, which **requires credentials**. The
tool uses the standard docker/podman auth files (`DOCKER_CONFIG`, `~/.docker/config.json`,
`REGISTRY_AUTH_FILE`). Log in with an app.ci session:

```bash
oc login https://api.ci.l2s4.p1.openshiftapps.com:6443 --web
oc registry login --registry quay-proxy.ci.openshift.org
```

Without credentials the report still runs. Only the "Built" column and the stale flag are missing.

The `.github/workflows/postsubmit-report.yaml` workflow runs it every 12h (or on demand).
Optional secrets: `QUAY_PROXY_AUTH` (a docker config.json with a quay-proxy entry) and
`POSTSUBMIT_SLACK_WEBHOOK_URL`.

## Architecture

The tool queries two data sources:

1. **OKD Release Controller** (`amd64.origin.releases.ci.openshift.org`) — for release stream tags, build phases, and job results
2. **GCS** (`test-platform-results` bucket) — for claude-payload-agent analysis artifacts (autodl.json and HTML summaries)

Both are public APIs requiring no authentication.
