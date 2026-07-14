package service

import (
	"context"
	"errors"
	"log/slog"
	"slices"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

var errNoIDInMessage = errors.New("no id in message")

func getUserID(update *models.Update) (fromID int64, err error) {
	if update.Message != nil && update.Message.From != nil {
		fromID = update.Message.From.ID
	} else if update.CallbackQuery != nil {
		fromID = update.CallbackQuery.From.ID
	} else {
		err = errNoIDInMessage
	}
	return
}

func EnrichContextWithUserID(next bot.HandlerFunc) bot.HandlerFunc {
	return func(ctx context.Context, bot *bot.Bot, update *models.Update) {
		// adding user id only when it's present
		if fromID, err := getUserID(update); err == nil {
			ctx = context.WithValue(ctx, "userID", fromID)
		}
		next(ctx, bot, update)
	}
}

func UsersWhitelistMiddleware(whitelist ...int64) func(bot.HandlerFunc) bot.HandlerFunc {
	return func(next bot.HandlerFunc) bot.HandlerFunc {
		return func(ctx context.Context, b *bot.Bot, update *models.Update) {
			fromID, err := getUserID(update)

			if err != nil {
				slog.DebugContext(ctx, "No identifiable ID in message in WhiteList middleware, dropping the message")
				return
			}

			if slices.Contains(whitelist, fromID) {
				next(ctx, b, update)
			}
		}
	}
}
