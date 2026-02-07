# li - LinkedIn CLI

A command-line interface for LinkedIn, inspired by `gh` (GitHub CLI).

## Features (planned)

- **Authentication**: OAuth or session-based login
- **Posts**: Create, view, like, comment on posts
- **Network**: Follow/unfollow, connect, view connections
- **Profile**: View and update profile
- **Search**: Search people, companies, jobs
- **Messages**: Send and read messages
- **Notifications**: View notifications

## Installation

```bash
go install github.com/horsefit/li@latest
```

## Usage

```bash
# Login
li auth login

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

## Development

```bash
git clone https://github.com/horsefit/li
cd li
go build
```

## License

MIT
