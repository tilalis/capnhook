package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/joho/godotenv"
	"github.com/tilalis/capnhook/media"
	"github.com/tilalis/capnhook/service"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Could not read .env")
	}

	slog.Info("Starting Capn' Hook service")

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	whitelist, err := parseWhitelist(os.Getenv("TELEGRAM_WHITELIST"))
	if err != nil {
		slog.ErrorContext(ctx, err.Error())
		return
	}

	maxSearchResults, err := parseMaxSearchResults(os.Getenv("SEARCH_MAX_RESULTS"))
	if err != nil {
		slog.ErrorContext(ctx, err.Error())
		return
	}

	mediaService, err := media.NewDefault(
		os.Getenv("PLEX_ROOT_DIR"),
		os.Getenv("TRANSMISSION_RPC_URL"),
		os.Getenv("SEARCH_CLIENT"),
		maxSearchResults,
	)
	if err != nil {
		slog.ErrorContext(ctx, err.Error())
		return
	}

	options := []bot.Option{
		bot.WithMiddlewares(
			service.UsersWhitelistMiddleware(whitelist...),
		),
		bot.WithDefaultHandler(
			service.QueryCommand(mediaService),
		),
		bot.WithCallbackQueryDataHandler(
			"id",
			bot.MatchTypePrefix,
			service.ShowTorrentInfoCallbackHandler(mediaService),
		),
		bot.WithCallbackQueryDataHandler(
			"download",
			bot.MatchTypePrefix,
			service.DownloadTorrentCallbackHandler(mediaService),
		),
		bot.WithCallbackQueryDataHandler(
			"torrent",
			bot.MatchTypePrefix,
			service.ManageTorrentCallbackHandler(mediaService),
		),
		bot.WithCallbackQueryDataHandler(
			"page",
			bot.MatchTypePrefix,
			service.TorrentsPageCallbackHandler(mediaService),
		),
		bot.WithCallbackQueryDataHandler(
			"deletetorrent",
			bot.MatchTypePrefix,
			service.DeleteTorrentCallbackHandler(mediaService),
		),
	}

	b, err := bot.New(os.Getenv("TELEGRAM_BOT_TOKEN"), options...)
	if err != nil {
		slog.ErrorContext(ctx, err.Error())
		return
	}

	infoCommand := service.InfoCommand(mediaService)
	b.RegisterHandler(
		bot.HandlerTypeMessageText,
		"info",
		bot.MatchTypeCommandStartOnly,
		infoCommand,
	)
	b.RegisterHandler(
		bot.HandlerTypeMessageText,
		"inprogress",
		bot.MatchTypeCommandStartOnly,
		infoCommand,
	)
	b.RegisterHandler(
		bot.HandlerTypeMessageText,
		"space",
		bot.MatchTypeCommandStartOnly,
		service.SpaceCommand(mediaService),
	)

	slog.Info("Capn' Hook service is up and running!")
	b.Start(ctx)
	slog.Info("The service is shutting down...")
}

// parseWhitelist parses a comma-separated list of Telegram user IDs from the
// TELEGRAM_WHITELIST env var.
func parseWhitelist(raw string) ([]int64, error) {
	var ids []int64
	for field := range strings.SplitSeq(raw, ",") {
		field = strings.TrimSpace(field)
		if field == "" {
			continue
		}

		id, err := strconv.ParseInt(field, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid whitelist id %q: %w", field, err)
		}

		ids = append(ids, id)
	}

	if len(ids) == 0 {
		return nil, errors.New("TELEGRAM_WHITELIST is empty")
	}

	return ids, nil
}

// parseMaxSearchResults parses the optional SEARCH_MAX_RESULTS env var. An empty
// value yields 0, letting the media package apply its own default.
func parseMaxSearchResults(raw string) (int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, nil
	}

	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("invalid SEARCH_MAX_RESULTS %q: %w", raw, err)
	}

	if n <= 0 {
		return 0, fmt.Errorf("SEARCH_MAX_RESULTS must be positive, got %d", n)
	}

	return n, nil
}
