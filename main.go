package main

import (
	"context"
	"log"
	"log/slog"
	"os"
	"os/signal"

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

	media, err := media.NewDefault()
	if err != nil {
		slog.ErrorContext(ctx, err.Error())
		return
	}

	options := []bot.Option{
		bot.WithMiddlewares(
			service.UsersWhitelistMiddleware(10965911), // whlstn
		),
		bot.WithDefaultHandler(
			service.QueryCommand(media),
		),
		bot.WithCallbackQueryDataHandler(
			"id",
			bot.MatchTypePrefix,
			service.ShowTorrentInfoCallbackHandler(media),
		),
		bot.WithCallbackQueryDataHandler(
			"download",
			bot.MatchTypePrefix,
			service.DownloadTorrentCallbackHandler(media),
		),
		bot.WithCallbackQueryDataHandler(
			"torrent",
			bot.MatchTypePrefix,
			service.ManageTorrentCallbackHandler(media),
		),
		bot.WithCallbackQueryDataHandler(
			"deletetorrent",
			bot.MatchTypePrefix,
			service.DeleteTorrentCallbackHandler(media),
		),
	}

	b, err := bot.New(os.Getenv("TELEGRAM_BOT_TOKEN"), options...)
	if err != nil {
		slog.ErrorContext(ctx, err.Error())
		return
	}

	infoCommand := service.InfoCommand(media)
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
		service.SpaceCommand(media),
	)

	slog.Info("Capn' Hook service is up and running!")
	b.Start(ctx)
	slog.Info("The service is shutting down...")
}
