package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/tilalis/capnhook/media"
)

// handles download* callbacks
func DownloadTorrentCallbackHandler(m *media.Media) bot.HandlerFunc {
	return func(ctx context.Context, bot *bot.Bot, update *models.Update) {
		sender := &callbackMessageSender{
			messageSender: messageSender{ctx, bot, update},
		}
		id, data, err := sender.parseCallback(3)

		if err != nil {
			sender.sendError(err)
			return
		}

		destination := data[2]

		torrentName, downloadDir, err := m.DownloadTorrent(ctx, id, destination)
		if err != nil {
			slog.ErrorContext(ctx, err.Error())
			sender.sendError(err)
			return
		}

		if err := sender.sendMessage(fmt.Sprintf("📥 <i>Downloading %s to %s</i>", torrentName, filepath.Base(downloadDir))); err != nil {
			slog.ErrorContext(ctx, err.Error())
		}
	}
}

// handles deletetorrent* callbacks
func DeleteTorrentCallbackHandler(m *media.Media) bot.HandlerFunc {
	return func(ctx context.Context, bot *bot.Bot, update *models.Update) {
		sender := &callbackMessageSender{
			messageSender: messageSender{ctx, bot, update},
		}

		if strings.HasPrefix(update.CallbackQuery.Data, "deletetorrentback") {
			sender.answerCallbackQuery()
			message := update.CallbackQuery.Message.Message

			if message == nil {
				slog.DebugContext(ctx, "No message in CallbackQuery on DeleteTorrentCallback, can't execute InfoCommand")
				return
			}

			update.Message = message
			InfoCommand(m)(ctx, bot, update)
			return
		}

		id, _, err := sender.parseCallback(2)

		if err != nil {
			slog.ErrorContext(ctx, err.Error())
			sender.sendError(err)
			return
		}

		torrentName, err := m.DeleteCurrentTorrent(ctx, id)
		if err != nil {
			sender.sendError(err)
			return
		}

		if err := sender.sendMessage(fmt.Sprintf("🗑️ Deleted with all files: %s", torrentName)); err != nil {
			slog.ErrorContext(ctx, err.Error())
		}
	}
}

// handles id* callbacks
func ShowTorrentInfoCallbackHandler(m *media.Media) bot.HandlerFunc {
	return func(ctx context.Context, bot *bot.Bot, update *models.Update) {
		sender := &callbackMessageSender{
			messageSender: messageSender{ctx, bot, update},
		}
		id, _, err := sender.parseCallback(2)
		if err != nil {
			slog.ErrorContext(ctx, err.Error())
			return
		}

		torrent, err := m.FindTorrent(ctx, id)
		if err != nil {
			slog.ErrorContext(ctx, err.Error())
			sender.sendError(err)
			return
		}

		description := fmt.Sprintf(
			"<a href=\"%s\">%s</a>\n<code>%.2fGB | %d files | %s | by %s</code>\n\n<blockquote>%s</blockquote>",
			m.SiteUrl(torrent),
			torrent.Name(),
			torrent.SizeGB(),
			torrent.NumFiles(),
			torrent.AddedTime().Format("2006-01-02"),
			torrent.Username(),
			torrent.Description(),
		)

		keyboard := [][]models.InlineKeyboardButton{
			{{Text: "📺 Download to Movies", CallbackData: fmt.Sprintf("download:%s:movies", torrent.ID())}},
			{{Text: "🎬 Download to TVShows", CallbackData: fmt.Sprintf("download:%s:tvshows", torrent.ID())}},
		}

		if err := sender.sendMessageWithKeyboard(description, keyboard); err != nil {
			slog.ErrorContext(ctx, err.Error())
		}
	}
}

// handles torrent* callbacks
func ManageTorrentCallbackHandler(m *media.Media) bot.HandlerFunc {
	return func(ctx context.Context, bot *bot.Bot, update *models.Update) {
		sender := &callbackMessageSender{
			messageSender: messageSender{ctx, bot, update},
		}
		id, _, err := sender.parseCallback(2)
		if err != nil {
			slog.ErrorContext(ctx, err.Error())
			return
		}

		torrentStatus, err := m.GetCurrentTorrent(ctx, id)
		if err != nil {
			slog.ErrorContext(ctx, err.Error())
			return
		}

		description := fmt.Sprintf(
			"%s <code>(%.0f%%, %s)</code>",
			torrentStatus.Name,
			torrentStatus.PercentDone,
			torrentStatus.SizeWhenDone.GiBString(),
		)

		inlineKeyboardButtons := [][]models.InlineKeyboardButton{
			{{Text: "🚫 Delete with all files", CallbackData: fmt.Sprintf("deletetorrent:%d", torrentStatus.ID)}},
			{{Text: "⬅️ Back", CallbackData: "deletetorrentback"}},
		}

		if err := sender.sendMessageWithKeyboard(description, inlineKeyboardButtons); err != nil {
			slog.ErrorContext(ctx, err.Error())
		}
	}
}

type callbackMessageSender struct {
	messageSender
	answered bool
}

func (d *callbackMessageSender) answerCallbackQuery() {
	if _, err := d.bot.AnswerCallbackQuery(d.ctx, &bot.AnswerCallbackQueryParams{
		CallbackQueryID: d.update.CallbackQuery.ID,
		ShowAlert:       false,
	}); err != nil {
		slog.ErrorContext(d.ctx, "failed to answer callback query", "error", err)
	}

	if _, err := d.bot.DeleteMessage(d.ctx, &bot.DeleteMessageParams{
		ChatID:    d.update.CallbackQuery.Message.Message.Chat.ID,
		MessageID: d.update.CallbackQuery.Message.Message.ID,
	}); err != nil {
		slog.ErrorContext(d.ctx, "failed to delete message", "error", err)
	}

	d.answered = true
}

func (d *callbackMessageSender) parseCallback(n int) (string, []string, error) {
	if !d.answered {
		d.answerCallbackQuery()
	}

	data := strings.SplitN(d.update.CallbackQuery.Data, ":", n)

	if len(data) < n {
		if _, err := d.bot.SendMessage(d.ctx, &bot.SendMessageParams{
			ChatID: d.update.CallbackQuery.Message.Message.Chat.ID,
			Text:   "Something went wrong :(",
		}); err != nil {
			slog.ErrorContext(d.ctx, "failed to send message", "error", err)
		}
		return "", nil, errors.New("can't parse callback data")
	}

	return data[1], data, nil
}
