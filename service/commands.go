package service

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/tilalis/capnhook/media"
	"github.com/tilalis/capnhook/service/paginator"
)

// handles general text messages
func QueryCommand(m *media.Media) bot.HandlerFunc {
	return func(ctx context.Context, bot *bot.Bot, update *models.Update) {
		if update.Message == nil {
			slog.ErrorContext(ctx, "Empty update")
			return
		}

		query := update.Message.Text
		torrents, err := m.SearchTorrents(ctx, query)
		sender := &messageSender{ctx, bot, update}

		if err != nil {
			slog.ErrorContext(ctx, err.Error())
			sender.sendError(err)
			return
		}

		torrentsPaginator, _ := paginator.NewPaginator(torrents, 5, 0)
		err = sendTorrentsMessage(sender, torrentsPaginator, query)

		if err != nil {
			slog.ErrorContext(ctx, err.Error())
			return
		}
	}
}

// handles /info and /inprogress commands
func InfoCommand(m *media.Media) bot.HandlerFunc {
	return func(ctx context.Context, bot *bot.Bot, update *models.Update) {
		inProgress := strings.HasPrefix(update.Message.Text, "/inprogress")
		torrentStatuses, err := m.GetCurrentTorrents(ctx, inProgress)

		var (
			responseStringBuilder strings.Builder
			responseKeyboard      [][]models.InlineKeyboardButton
		)

		for i, torrentStatus := range torrentStatuses {
			var status string
			if torrentStatus.Done {
				status = "✅"
			} else {
				status = "🔄"
			}

			idx := i + 1
			fmt.Fprintf(
				&responseStringBuilder,
				"%d: %s %s <code>(%.0f%%, %.2fh, %s)</code>\n",
				idx,
				status,
				torrentStatus.Name,
				torrentStatus.PercentDone,
				torrentStatus.ETA.Hours(),
				torrentStatus.SizeWhenDone.GiBString(),
			)
			responseKeyboard = append(responseKeyboard, []models.InlineKeyboardButton{
				{
					Text:         fmt.Sprintf("%d: %s", idx, torrentStatus.Name),
					CallbackData: fmt.Sprintf("torrent:%d", torrentStatus.ID),
				},
			})
		}

		sender := &messageSender{ctx, bot, update}

		if err != nil {
			slog.ErrorContext(ctx, err.Error())
			sender.sendError(err)
			return
		}

		responseText := responseStringBuilder.String()

		if responseText == "" {
			if err := sender.sendMessage("<i>No downloads in progress</i>"); err != nil {
				slog.ErrorContext(ctx, err.Error())
			}
			return
		}

		if err := sender.sendMessageWithKeyboard(
			responseText,
			responseKeyboard,
		); err != nil {
			slog.ErrorContext(ctx, err.Error())
		}
	}
}

// handles /space command
func SpaceCommand(m *media.Media) bot.HandlerFunc {
	return func(ctx context.Context, bot *bot.Bot, update *models.Update) {
		freeSpace, err := m.FreeSpace(ctx)
		sender := &messageSender{ctx, bot, update}

		if err != nil {
			slog.ErrorContext(ctx, err.Error())
			sender.sendError(err)
			return
		}

		err = sender.sendMessage(fmt.Sprintf("💾 Free Space: %s", freeSpace.GiBString()))

		if err != nil {
			slog.ErrorContext(ctx, err.Error())
			return
		}
	}
}
