package service

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/tilalis/capnhook/media"
	"github.com/tilalis/capnhook/service/paginator"
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

const maximumDescriptionLength = 3072

// handles id* callbacks
func ShowTorrentInfoCallbackHandler(m *media.Media) bot.HandlerFunc {
	return func(ctx context.Context, bot *bot.Bot, update *models.Update) {
		sender := &callbackMessageSender{
			messageSender: messageSender{ctx, bot, update},
		}
		id, data, err := sender.parseCallback(3)
		if err != nil {
			slog.ErrorContext(ctx, err.Error())
			return
		}

		query := data[2]
		torrent, err := m.FindTorrent(ctx, id)
		if err != nil {
			slog.ErrorContext(ctx, err.Error())
			sender.sendError(err)
			return
		}

		descriptionTextRunes := []rune(torrent.Description())
		if len(descriptionTextRunes) > maximumDescriptionLength {
			descriptionTextRunes = descriptionTextRunes[:maximumDescriptionLength]
		}

		description := fmt.Sprintf(
			"<a href=\"%s\">%s</a>\n<blockquote>%.2fGB | %d files | %s | by %s</blockquote><blockquote expandable>%s</blockquote>",
			m.SiteUrl(torrent),
			torrent.Name(),
			torrent.SizeGB(),
			torrent.NumFiles(),
			torrent.AddedTime().Format("2006-01-02"),
			torrent.Username(),
			string(descriptionTextRunes),
		)

		keyboard := [][]models.InlineKeyboardButton{
			{{Text: "📺 Download to Movies", CallbackData: fmt.Sprintf("download:%s:movies", torrent.ID())}},
			{{Text: "🎬 Download to TVShows", CallbackData: fmt.Sprintf("download:%s:tvshows", torrent.ID())}},
			{{Text: "⏪ Back", CallbackData: fmt.Sprintf("page:0:%s", query)}},
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

// handles page* callback
func TorrentsPageCallbackHandler(m *media.Media) bot.HandlerFunc {
	return func(ctx context.Context, bot *bot.Bot, update *models.Update) {
		sender := &callbackMessageSender{
			messageSender: messageSender{ctx, bot, update},
		}

		pageNumberString, data, err := sender.parseCallback(3)

		if err != nil {
			slog.ErrorContext(ctx, err.Error())
			sender.sendError(err)
			return
		}

		query := data[2]
		pageNumber, err := strconv.Atoi(pageNumberString)

		if err != nil {
			slog.ErrorContext(ctx, err.Error())
			sender.sendError(err)
			return
		}

		// cached search
		torrents, err := m.SearchTorrents(ctx, query)

		if err != nil {
			slog.ErrorContext(ctx, err.Error())
			sender.sendError(err)
			return
		}

		torrentsPaginator, err := paginator.NewPaginator(torrents, 5, pageNumber)

		if err != nil {
			slog.ErrorContext(ctx, err.Error())
			sender.sendError(err)
			return
		}
		err = sendTorrentsMessage(sender, torrentsPaginator, query)
	}
}
