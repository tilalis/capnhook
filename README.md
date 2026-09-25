# Cap'n Hook 🪝

A personal Telegram bot for managing a home media server. 

Search the [Internet Archive](https://archive.org) from a chat, send torrents to a [Transmission](https://transmissionbt.com/) daemon, and route downloads straight into your Plex library — all from your phone, gated behind a user whitelist.

## Features

- 🔎 **Search** the Internet Archive for freely licensed movies by sending any text message
- 📥 **Download** a result into your `Movies` or `TVShows` folder with one tap
- 🧲 **Magnet links** are recognised and added directly, skipping the search
- 📊 **Track** progress, ETA and size of active downloads (`/info`, `/inprogress`)
- ⏸️ **Pause / resume** a torrent from a chat button, without losing it
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

Send a **magnet link** instead of a search query and the search step is skipped:
the bot reads the info hash out of the link and goes straight to the
**Download to Movies** / **Download to TVShows** buttons.
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
| `TELEGRAM_WEBHOOK_URL`    |          | —         | Public https url Telegram pushes updates to. Set it to serve in webhook mode; leave empty for long polling |
| `TELEGRAM_WEBHOOK_ADDR`   |          | `:8080`   | Address the webhook server listens on                              |
| `TELEGRAM_WEBHOOK_PATH`   |          | path of `TELEGRAM_WEBHOOK_URL` | Local path to serve the webhook on; only needed when a proxy rewrites the path |
| `TELEGRAM_WEBHOOK_SECRET` |          | —         | Shared secret validated against `X-Telegram-Bot-Api-Secret-Token`  |

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

### Serving modes

The bot receives updates either way — pick whichever fits your deployment:

- **Long polling** (default) — the bot asks Telegram for updates. Nothing has to
  be reachable from the internet, which makes it the easy choice on a home
  server. Any webhook registered by a previous run is removed on startup.
- **Webhook** — Telegram pushes updates to you. Set `TELEGRAM_WEBHOOK_URL` and
  the bot registers that url, then serves it on `TELEGRAM_WEBHOOK_ADDR` at the
  url's path. This needs a public **https** endpoint (Telegram refuses plain
  http), so put a TLS-terminating reverse proxy in front of the bot and forward
  to it.

```sh
TELEGRAM_WEBHOOK_URL=https://bot.example.com/telegram/hook
TELEGRAM_WEBHOOK_ADDR=:8080
TELEGRAM_WEBHOOK_SECRET=some-long-random-string
```

With the above, the bot serves `POST /telegram/hook` on port 8080 and rejects
any request whose `X-Telegram-Bot-Api-Secret-Token` header does not match the
secret. Set a secret whenever you expose the endpoint: the url alone is the only
thing standing between the internet and your update stream.

A request whose secret does not match is answered with `403`, so a broken setup
shows up in Telegram's own diagnostics:

```sh
curl -s "https://api.telegram.org/bot$TELEGRAM_BOT_TOKEN/getWebhookInfo"
```

A non-empty `last_error_message` there means Telegram is reaching the bot but
being turned away — most often a proxy that drops the
`X-Telegram-Bot-Api-Secret-Token` header.

On shutdown the webhook stays registered, so Telegram queues updates and
redelivers them once the bot is back. Switching back to long polling is just a
matter of clearing `TELEGRAM_WEBHOOK_URL` — the next start deregisters it.

## Usage

Once the bot is running and your user ID is whitelisted:

| Input                  | Action                                                        |
| ---------------------- | ------------------------------------------------------------- |
| _any text_             | Search torrents; results appear as tappable buttons           |
| _a magnet link_        | Skip the search and offer **Download to Movies / TVShows**    |
| tap a search result    | Show torrent details + **Download to Movies / TVShows**       |
| `/info`                | List all torrents in Transmission with manage/delete buttons  |
| tap a torrent there    | **Pause / Resume**, **Notify on progress**, **Delete**        |
| `/inprogress`          | Same as `/info`, but hides completed downloads                |
| `/space`               | Show free disk space on the media volume                      |

## Project layout

```
main.go                          Composition root: config, wiring, handler registration
serve.go                         Long polling / webhook serving modes
media/                           Orchestration facade over search + transmission, result cache
media/magnet.go                  Magnet link recognition and info hash parsing
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
