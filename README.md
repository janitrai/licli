# li - LinkedIn CLI

A command-line interface for LinkedIn, inspired by `gh` (GitHub CLI).

## Features

- **Authentication**: Browser-session login (stores `li_at` + `JSESSIONID`)
- **Posts**: Create and list posts
- **Network**: Follow and connect
- **Profile**: View profiles (including your own)
- **Search**: Search people and jobs

Note: this uses LinkedIn's internal Voyager API (`https://www.linkedin.com/voyager/api`) and may break if LinkedIn changes it.

## Installation

```bash
go install github.com/horsefit/li@latest
```

## Usage

```bash
# Login
li auth login
li auth status

# Post
li post create "Hello LinkedIn!"
li post list

# Network
li follow @username
li connect @username --note "Hey, let's connect!"

# Profile
li profile view @username
li profile me

# Search
li search people "software engineer berlin"
li search jobs "golang developer"
```

## Config

By default, config is stored at `$XDG_CONFIG_HOME/li/config.json` (Linux typically `~/.config/li/config.json`).

Override with:

```bash
export LI_CONFIG_PATH=/path/to/config.json
```

## Development

```bash
git clone https://github.com/horsefit/li
cd li
go build
```

## License

MIT
