# Cap'n Hook 🪝

A personal Telegram bot for managing a home media server. 

Search the [Internet Archive](https://archive.org) from a chat, send torrents to a [Transmission](https://transmissionbt.com/) daemon, and route downloads straight into your Plex library — all from your phone, gated behind a user whitelist.

## Features

- 🔎 **Search** the Internet Archive for freely licensed movies by sending any text message
- 📥 **Download** a result into your `Movies` or `TVShows` folder with one tap
- 📊 **Track** progress, ETA and size of active downloads (`/info`, `/inprogress`)
- 🗑️ **Delete** a torrent and its files from a chat button
- 💾 **Check** free disk space (`/space`)
- 🔒 **Whitelist** — only approved Telegram user IDs can talk to the bot
- 🧠 In-memory cache of looked-up torrents to avoid redundant API calls

## How it works

```
Telegram user ──▶ service (handlers, whitelist) ──▶ media (facade + cache)
                                                      ├─▶ search client 
                                                      └─▶ transmission (RPC daemon)
```

Typical flow: send a search query → pick a result from the inline keyboard → the
bot shows details with **Download to Movies** / **Download to TVShows** buttons →
the torrent is added to Transmission with the chosen download directory.

## Prerequisites

- Go **1.26+**
- A running **Transmission** daemon with the RPC endpoint reachable
- A **Telegram bot token** from [@BotFather](https://t.me/BotFather)
- Your **Telegram user ID** (e.g. from [@userinfobot](https://t.me/userinfobot))

## Configuration

Configuration is read from the environment (a local `.env` file is loaded on
startup). Copy the example and fill it in:

```sh
cp .env.example .env
```

| Variable               | Required | Default                                        | Description                                                        |
| ---------------------- | :------: | ---------------------------------------------- | ------------------------------------------------------------------ |
| `TELEGRAM_BOT_TOKEN`   |    ✅    | —                                              | Bot token from @BotFather                                          |
| `TELEGRAM_WHITELIST`   |    ✅    | —                                              | Comma-separated Telegram user IDs allowed to use the bot           |
| `TRANSMISSION_RPC_URL` |    ✅    | `http://192.168.1.42:9091/transmission/rpc`    | Transmission RPC endpoint                                          |
| `PLEX_ROOT_DIR`        |    ✅    | —                                              | Root media directory containing `Movies/` and `TVShows/`          |
| `SEARCH_MAX_RESULTS`   |          | `30`                                           | Max number of search results shown per query                       |

> **Note:** `.env` holds your bot token and is git-ignored. Never commit it. If a
> token is ever exposed, revoke it via @BotFather.

## Running

```sh
# from the repository root, with .env in place
go run .
```

Or build a binary:

```sh
go build -o capnhook .
./capnhook
```

The process handles `SIGINT` (Ctrl-C) for a clean shutdown.

## Usage

Once the bot is running and your user ID is whitelisted:

| Input                  | Action                                                        |
| ---------------------- | ------------------------------------------------------------- |
| _any text_             | Search torrents; results appear as tappable buttons           |
| tap a search result    | Show torrent details + **Download to Movies / TVShows**       |
| `/info`                | List all torrents in Transmission with manage/delete buttons  |
| `/inprogress`          | Same as `/info`, but hides completed downloads                |
| `/space`               | Show free disk space on the media volume                      |

## Project layout

```
main.go                          Composition root: config, wiring, handler registration
media/                           Orchestration facade over search + transmission, result cache
media/interfaces/                Search client interface
media/clients/internetarchive/   archive.org search client (CC / public domain movies)
media/clients/apibay/            apibay.org search client
media/transmission/              Transmission RPC client wrapper
service/                         Telegram command/callback handlers, whitelist middleware
```

## Testing

```sh
go test ./...
```

## Tech stack

- [go-telegram/bot](https://github.com/go-telegram/bot) — Telegram Bot API
- [hekmon/transmissionrpc](https://github.com/hekmon/transmissionrpc) — Transmission RPC
- [joho/godotenv](https://github.com/joho/godotenv) — `.env` loading
- [scalalang2/golang-fifo](https://github.com/scalalang2/golang-fifo) — SIEVE cache
