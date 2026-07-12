package service

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/tilalis/capnhook/media"
)

// handles general text messages
func QueryCommand(m *media.Media) bot.HandlerFunc {
	return func(ctx context.Context, bot *bot.Bot, update *models.Update) {
		if update.Message == nil {
			slog.ErrorContext(ctx, "Empty update")
			return
		}

		torrents, err := m.SearchTorrents(ctx, update.Message.Text)
		sender := &messageSender{ctx, bot, update}

		if err != nil {
			slog.ErrorContext(ctx, err.Error())
			sender.sendError(err)
			return
		}
		var (
			responseText     strings.Builder
			responseKeyboard [][]models.InlineKeyboardButton = make([][]models.InlineKeyboardButton, 0, m.MaxSearchResults())
		)

		for i, torrent := range torrents {
			idx := i + 1
			fmt.Fprintf(
				&responseText,
				"<b>%d</b>: <code>%s</code>\n%.2fGB, %d files, %s by %s\n\n",
				idx,
				torrent.Name(),
				torrent.SizeGB(),
				torrent.NumFiles(),
				torrent.AddedTime().Format("2006-01-02"),
				torrent.Username(),
			)

			responseKeyboard = append(
				responseKeyboard,
				[]models.InlineKeyboardButton{
					{Text: fmt.Sprintf("%d: %s", idx, torrent.Name()), CallbackData: fmt.Sprintf("id:%s", torrent.ID())},
				},
			)
		}

		err = sender.sendMessageWithKeyboard(responseText.String(), responseKeyboard)

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
