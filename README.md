<p align="center">
  <h1 align="center">ecsx</h1>
  <p align="center">
    <em>Your ECS clusters, one keystroke away.</em>
  </p>
  <p align="center">
    <a href="#installation">Installation</a> •
    <a href="#usage">Usage</a> •
    <a href="#keybindings">Keybindings</a>
  </p>
</p>

---

**ecsx** is a terminal UI for Amazon ECS. Browse clusters, services, and tasks
in a split-pane interface. Tail logs, scale services, exec into containers, and
open SSM sessions — without leaving your terminal.

## ✨ Highlights

- 🗂 **Browse** clusters → services → tasks with fuzzy filtering
- 📊 **Metrics** — CPU and memory sparklines per service
- 📜 **Logs** — real-time CloudWatch log tailing with filter patterns
- 🪵 **humanlog** — pipe logs through [`hl`](https://github.com/humanlogio/humanlog) for structured log formatting
- 📝 **Editor** — open buffered logs in `$EDITOR` for search and analysis
- ⚖️ **Scale** — update desired count on the fly
- 🛑 **Stop** — stop individual tasks
- 🔑 **Env vars** — inspect container environment, copy to clipboard
- 🐚 **Exec** — shell into running containers via ECS ExecuteCommand
- 🔌 **SSM** — connect to EC2 container instances
- ⌨️ **Completions** — bash, zsh, fish, powershell

## Prerequisites

- AWS credentials configured (via CLI, env vars, or IAM role)
- [session-manager-plugin](https://docs.aws.amazon.com/systems-manager/latest/userguide/session-manager-working-with-install-plugin.html)
  — required for `exec` and `ssm` commands

For `exec` (the `x` keybinding in the tasks view) the target ECS service must have
`enableExecuteCommand: true` set on its service definition **before** tasks are
launched — the flag does not apply to tasks already running. If exec fails with
`InvalidParameterException: ... execute command was not enabled when the task
was run`, set the flag and roll new tasks. The container also needs the SSM
agent reachable (Fargate has this by default; EC2 launch type requires the
ecs-agent on the instance to be configured for SSM).

## Installation

**Go:**

```sh
go install github.com/rvdwijngaard/ecsx/cmd/ecsx@latest
```

**From source:**

```sh
git clone https://github.com/rvdwijngaard/ecsx.git
cd ecsx
make install
```

## Usage

```sh
ecsx                          # launch TUI, browse all clusters
ecsx -c my-cluster            # jump straight to a cluster
ecsx -p prod -r eu-west-1     # specify AWS profile and region
```

## Keybindings

| Key     | Action                          |
| ------- | ------------------------------- |
| `enter` | Drill into selected item        |
| `esc`   | Go back                         |
| `/`     | Filter list or logs             |
| `f`     | Cycle log level (in logs)       |
| `g`     | Regex grep filter (in logs)     |
| `l`     | Tail logs                       |
| `e`     | Open logs in `$EDITOR` / toggle env vars |
| `y`     | Copy env vars to clipboard      |
| `s`     | Scale service                   |
| `x`     | Exec into container / open host shell |
| `o`     | Open in AWS web console               |
| `r`     | Refresh                         |
| `+` `-` | Toggle zoom                     |
| `?`     | Help                            |
| `q`     | Quit                            |

## Global Flags

| Flag        | Short | Description                  |
| ----------- | ----- | ---------------------------- |
| `--profile` | `-p`  | AWS profile                  |
| `--region`  | `-r`  | AWS region                   |
| `--cluster` | `-c`  | ECS cluster (skip selection) |

## Configuration

ecsx reads its config from `~/.config/ecsx/config.yaml` (created on first run).

```yaml
default_region: eu-west-1
starred_regions:
  - eu-west-1
  - us-east-1
logs_viewer: "hl -L -F"
```

| Key               | Description                                                                 |
| ----------------- | --------------------------------------------------------------------------- |
| `default_region`  | Fallback region when `--region` and `AWS_REGION` are unset                  |
| `default_profile` | Fallback AWS profile                                                        |
| `starred_regions` | Pinned to the top of the region picker                                      |
| `aws_regions`     | Override the built-in region list                                            |
| `logs_viewer`     | External command to pipe logs through (e.g. `hl -L -F` for humanlog)        |

### humanlog integration

When `logs_viewer` is set, pressing `l` on a service suspends the TUI and pipes
CloudWatch logs directly into the configured command. This is ideal for
[`hl`](https://github.com/humanlogio/humanlog) which renders structured
(JSON/logfmt) logs with colors, timestamps, and key highlighting. Press
`Ctrl+C` to return to the TUI.

## IAM Permissions

By default, ecsx is non-destructive — browsing clusters, services, tasks, logs,
and metrics requires only read-only permissions. Write actions (scale, deploy,
stop, exec, ssm) are opt-in and triggered explicitly by the user.

### Read-only (browsing, logs, metrics)

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "ecs:ListClusters",
        "ecs:DescribeClusters",
        "ecs:ListServices",
        "ecs:DescribeServices",
        "ecs:ListTasks",
        "ecs:DescribeTasks",
        "ecs:ListContainerInstances",
        "ecs:DescribeContainerInstances",
        "ecs:DescribeTaskDefinition",
        "cloudwatch:GetMetricStatistics",
        "logs:DescribeLogGroups",
        "logs:StartLiveTail",
        "logs:FilterLogEvents"
      ],
      "Resource": "*"
    }
  ]
}
```

### Write actions (scale, deploy, stop)

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "ecs:UpdateService",
        "ecs:StopTask"
      ],
      "Resource": "*"
    }
  ]
}
```

### ECS Exec and SSM sessions

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "ecs:ExecuteCommand"
      ],
      "Resource": "*"
    },
    {
      "Effect": "Allow",
      "Action": [
        "ssm:StartSession"
      ],
      "Resource": "*"
    }
  ]
}
```

## Inspiration

Built on the shoulders of these great tools:

- [dynamite](https://github.com/wolfwfr/dynamite) — the TUI experience is a shameless copy of this awesome DynamoDB tool
- [ecsq](https://github.com/mightyguava/ecsq) — friendly ECS CLI for querying clusters, services, and tasks
- [cw](https://github.com/lucagrulla/cw) — CloudWatch Logs tail from the terminal
- [gossm](https://github.com/gjbae1212/gossm) — interactive SSM session manager

## License

MIT
