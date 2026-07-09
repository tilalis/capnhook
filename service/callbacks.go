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

		sender.sendMessage(fmt.Sprintf("📥 <i>Downloading %s to %s</i>", torrentName, filepath.Base(downloadDir)))
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

		sender.sendMessage(fmt.Sprintf("🗑️ Deleted with all files: %s", torrentName))
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
			torrent.SiteUrl,
			torrent.Name,
			torrent.SizeGB,
			torrent.NumFiles,
			torrent.Added.Format("2006-01-02"),
			torrent.Username,
			torrent.Description,
		)

		keyboard := [][]models.InlineKeyboardButton{
			{{Text: "📺 Download to Movies", CallbackData: fmt.Sprintf("download:%s:movies", torrent.ID)}},
			{{Text: "🎬 Download to TVShows", CallbackData: fmt.Sprintf("download:%s:tvshows", torrent.ID)}},
		}

		sender.sendMessageWithKeyboard(description, keyboard)
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

		sender.sendMessageWithKeyboard(description, inlineKeyboardButtons)
	}
}

type callbackMessageSender struct {
	messageSender
	answered bool
}

func (d *callbackMessageSender) answerCallbackQuery() {
	d.bot.AnswerCallbackQuery(d.ctx, &bot.AnswerCallbackQueryParams{
		CallbackQueryID: d.update.CallbackQuery.ID,
		ShowAlert:       false,
	})

	d.bot.DeleteMessage(d.ctx, &bot.DeleteMessageParams{
		ChatID:    d.update.CallbackQuery.Message.Message.Chat.ID,
		MessageID: d.update.CallbackQuery.Message.Message.ID,
	})

	d.answered = true
}

func (d *callbackMessageSender) parseCallback(n int) (string, []string, error) {
	if !d.answered {
		d.answerCallbackQuery()
	}

	data := strings.SplitN(d.update.CallbackQuery.Data, ":", n)

	if len(data) < n {
		d.bot.SendMessage(d.ctx, &bot.SendMessageParams{
			ChatID: d.update.CallbackQuery.Message.Message.Chat.ID,
			Text:   "Something went wrong :(",
		})
		return "", nil, errors.New("can't parse callback data")
	}

	return data[1], data, nil
}
