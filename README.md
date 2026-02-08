# bragcli - Bragnet CLI

A command-line interface for Bragnet, inspired by `gh` (GitHub CLI).

## Features

- **Authentication**: Browser-session login (stores session cookies)
- **Posts**: Create and list posts
- **Network**: Follow and connect
- **Profile**: View profiles (including your own)
- **Search**: Search people and jobs
- **Messaging**: Read and send messages

## Installation

```bash
go install github.com/janitrai/licli/cmd/bragcli@latest
```

## Configuration

Set the target domain via environment variable:

```bash
export BRAGNET_DOMAIN="www.example.com"
```

## Usage

```bash
# Login
bragcli auth login
bragcli auth status

# Post
bragcli post create "Hello world!"
bragcli post list

# Network
bragcli follow @username
bragcli connect @username --note "Hey, let's connect!"

# Profile
bragcli profile view @username
bragcli profile me

# Search
bragcli search people "software engineer berlin"
bragcli search jobs "golang developer"

# Messaging
bragcli message list
bragcli message read @username
bragcli message send @username "Hey there!"
```

## Config

By default, config is stored at `$XDG_CONFIG_HOME/li/config.json` (Linux typically `~/.config/li/config.json`).

Override with:

```bash
export LI_CONFIG_PATH=/path/to/config.json
```

## Development

```bash
git clone https://github.com/janitrai/licli
cd licli
go build ./cmd/bragcli
```

## License

MIT
