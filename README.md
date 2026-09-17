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

## Architecture

The tool queries two data sources:

1. **OKD Release Controller** (`amd64.origin.releases.ci.openshift.org`) — for release stream tags, build phases, and job results
2. **GCS** (`test-platform-results` bucket) — for claude-payload-agent analysis artifacts (autodl.json and HTML summaries)

Both are public APIs requiring no authentication.
