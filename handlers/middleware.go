package handlers

import (
	"context"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// const []int64 whiteList = {10965911}

func usersWhitelist(next bot.HandlerFunc) bot.HandlerFunc {
	return func(ctx context.Context, b *bot.Bot, update *models.Update) {
		var fromID int64

		if update.Message != nil {
			fromID = update.Message.From.ID
		} else {
			fromID = update.CallbackQuery.From.ID
		}

		// whlstn only
		if fromID == 10965911 {
			next(ctx, b, update)
		}
	}
}
