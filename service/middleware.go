package service

import (
	"context"
	"log/slog"
	"slices"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

func UsersWhitelistMiddleware(whitelist ...int64) func(bot.HandlerFunc) bot.HandlerFunc {
	return func(next bot.HandlerFunc) bot.HandlerFunc {
		return func(ctx context.Context, b *bot.Bot, update *models.Update) {
			var fromID int64

			if update.Message != nil && update.Message.From != nil {
				fromID = update.Message.From.ID
			} else if update.CallbackQuery != nil {
				fromID = update.CallbackQuery.From.ID
			} else {
				slog.DebugContext(ctx, "No identifiable ID in message in WhiteList middleware, dropping the message")
				return
			}

			if slices.Contains(whitelist, fromID) {
				next(ctx, b, update)
			}
		}
	}
}
