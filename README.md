# jw

<p align="center">
  <img src="jw.png" alt="jw" width="300">
</p>

A CLI tool that monitors Jenkins jobs in the background and sends macOS notifications when they complete.

## Installation

### Homebrew (Recommended)

```bash
brew install baggiiiie/tap/jw
```

### From Go

```bash
go install github.com/baggiiiie/jw@latest
```

Or build from source:

```bash
go build -o jw .
```

## Usage

Set your Jenkins credentials (same format as `curl -u user:token`):

```bash
export JENKINS_USER=your_username
export JENKINS_API_TOKEN=your_api_token
```

Or use the legacy pre-encoded token:

```bash
export JENKINS_TOKEN=base64_encoded_credentials
```

Add a job to monitor:

```bash
jw add https://jenkins.example.com/job/my-job/123/
```

Check status:

```bash
jw status
```

Other commands:

```bash
jw remove <job_url>   # Stop monitoring a job
jw stop               # Stop the daemon
jw logs               # View daemon logs
jw status --tui       # Interactive TUI
```

## Architecture

```mermaid
graph TD
    CLI["CLI Commands<br/>add · remove · status · stop · logs · auth"]
    Daemon["Daemon<br/>signal-driven event loop<br/>SIGHUP reload · SIGTERM shutdown"]
    Monitor["Monitor<br/>poll every 30s per job"]
    Config["Config Store<br/>file-locked read-modify-write"]
    Notify["Notifier<br/>macOS notifications"]
    PID["PID File<br/>self-healing"]

    Jenkins["Jenkins API"]
    FS["~/.jw/"]
    macOS["terminal-notifier<br/>/ osascript"]

    CLI -->|"spawn / SIGHUP"| Daemon
    CLI -->|"read/write jobs"| Config
    Daemon -->|"goroutine per job"| Monitor
    Monitor -->|"JobEvent channel"| Daemon
    Monitor -->|"HTTP poll"| Jenkins
    Daemon -->|"on completion"| Notify
    Daemon -->|"tick 5s"| PID
    Config -->|"flock"| FS
    PID --> FS
    Notify --> macOS

    classDef internal fill:#4a9eff,color:#fff
    classDef ext fill:#f87171,color:#fff
    class CLI,Daemon,Monitor,Config,Notify,PID internal
    class Jenkins,FS,macOS ext
```

## License

MIT
