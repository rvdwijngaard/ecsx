<p align="center">
  <h1 align="center">ecsx</h1>
  <p align="center">
    <em>Your ECS clusters, one keystroke away.</em>
  </p>
  <p align="center">
    <a href="#installation">Installation</a> •
    <a href="#usage">Usage</a> •
    <a href="#commands">Commands</a> •
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
- 📝 **Editor** — open buffered logs in `$EDITOR` for search and analysis
- ⚖️ **Scale** — update desired count on the fly
- 🔑 **Env vars** — inspect container environment, copy to clipboard
- 🐚 **Exec** — shell into running containers via ECS ExecuteCommand
- 🔌 **SSM** — connect to EC2 container instances
- ⌨️ **Completions** — bash, zsh, fish, powershell

## Prerequisites

- AWS credentials configured (via CLI, env vars, or IAM role)
- [session-manager-plugin](https://docs.aws.amazon.com/systems-manager/latest/userguide/session-manager-working-with-install-plugin.html)
  — required for `exec` and `ssm` commands

## Installation

**Go:**

```sh
go install github.com/ron/ecsx/cmd/ecsx@latest
```

**From source:**

```sh
git clone https://github.com/ron/ecsx.git
cd ecsx
make install
```

## Usage

```sh
ecsx                          # launch TUI, browse all clusters
ecsx -c my-cluster            # jump straight to a cluster
ecsx -p prod -r eu-west-1     # specify AWS profile and region
```

## Commands

### `ecsx logs` — tail CloudWatch logs

```sh
ecsx logs -c my-cluster -s my-service
ecsx logs -c my-cluster -s my-service -f "ERROR"         # filter pattern
ecsx logs -c my-cluster -s my-service -b 2h              # last 2 hours
ecsx logs -c my-cluster -s my-service --no-follow        # dump and exit
```

### `ecsx exec` — shell into a container

```sh
ecsx exec -c my-cluster -s my-service                    # /bin/sh into first task
ecsx exec -c my-cluster -s my-service --cmd "ls -la"     # run a command
ecsx exec -c my-cluster -t <task-id> -u my-container     # target specific task + container
```

> Requires `session-manager-plugin` and
> [`enableExecuteCommand`](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/ecs-exec.html)
> enabled on the ECS service.

### `ecsx ssm` — connect to EC2 instances

```sh
ecsx ssm -c my-cluster -s my-service     # resolve instance from service tasks
ecsx ssm -c my-cluster -i i-0abc123      # connect directly
```

### `ecsx completion` — shell completions

```sh
ecsx completion bash | sudo tee /etc/bash_completion.d/ecsx
ecsx completion zsh > ~/.zfunc/_ecsx
ecsx completion fish > ~/.config/fish/completions/ecsx.fish
```

## Keybindings

| Key     | Action                          |
| ------- | ------------------------------- |
| `enter` | Drill into selected item        |
| `esc`   | Go back                         |
| `/`     | Filter list or logs             |
| `l`     | Tail logs                       |
| `e`     | Open logs in `$EDITOR` / toggle env vars |
| `y`     | Copy env vars to clipboard      |
| `s`     | Scale service                   |
| `x`     | Open SSM session                |
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

## Inspiration

Built on the shoulders of these great tools:

- [ecsq](https://github.com/mightyguava/ecsq) — friendly ECS CLI for querying clusters, services, and tasks
- [cw](https://github.com/lucagrulla/cw) — CloudWatch Logs tail from the terminal
- [gossm](https://github.com/gjbae1212/gossm) — interactive SSM session manager

## License

MIT
