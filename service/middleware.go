package service

import (
	"context"
	"log/slog"
	"slices"
	"strconv"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

func EnrichContextWithUserID(next bot.HandlerFunc) bot.HandlerFunc {
	return func(ctx context.Context, bot *bot.Bot, update *models.Update) {
		// adding user id only when it's present
		var fromID int64

		if update.Message != nil && update.Message.From != nil {
			fromID = update.Message.From.ID
		} else if update.CallbackQuery != nil {
			fromID = update.CallbackQuery.From.ID
		} else {
			next(ctx, bot, update)
			return
		}
		
		next(context.WithValue(ctx, "userID", strconv.FormatInt(fromID, 10)), bot, update)
	}
}

func UsersWhitelistMiddleware(whitelist ...string) func(bot.HandlerFunc) bot.HandlerFunc {
	return func(next bot.HandlerFunc) bot.HandlerFunc {
		return func(ctx context.Context, b *bot.Bot, update *models.Update) {
			userID, ok := ctx.Value("userID").(string)

			if !ok {
				slog.DebugContext(ctx, "No ID in message in WhiteList middleware, dropping the message")
				return
			}

			if slices.Contains(whitelist, userID) {
				next(ctx, b, update)
			}
		}
	}
}
