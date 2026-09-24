package service

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

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

// handles magnet* callbacks
func DownloadMagnetCallbackHandler(m *media.Media) bot.HandlerFunc {
	return func(ctx context.Context, bot *bot.Bot, update *models.Update) {
		sender := &callbackMessageSender{
			messageSender: messageSender{ctx, bot, update},
		}
		infoHash, data, err := sender.parseCallback(3)

		if err != nil {
			sender.sendError(err)
			return
		}

		destination := data[2]

		torrentName, downloadDir, err := m.DownloadMagnet(ctx, infoHash, destination)
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
			{{
				Text:         "📺 Download to Movies",
				Style:        styleSuccess,
				CallbackData: fmt.Sprintf("download:%s:movies", torrent.ID()),
			}},
			{{
				Text:         "🎬 Download to TVShows",
				Style:        styleSuccess,
				CallbackData: fmt.Sprintf("download:%s:tvshows", torrent.ID()),
			}},
			{{
				Text:         "⏪ Back",
				Style:        stylePrimary,
				CallbackData: fmt.Sprintf("page:0:%s", query),
			}},
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
			{{
				Text:         "🚫 Delete with all files",
				Style:        styleDanger,
				CallbackData: fmt.Sprintf("deletetorrent:%d", torrentStatus.ID),
			}},
			{{
				Text:         "⬅️ Back",
				Style:        stylePrimary,
				CallbackData: "deletetorrentback",
			}},
		}

		if !torrentStatus.Done {
			inlineKeyboardButtons = append(
				[][]models.InlineKeyboardButton{
					{{
						Text:         "🔔 Notify on progress",
						Style:        styleSuccess,
						CallbackData: fmt.Sprintf("track:%d", torrentStatus.ID),
					}},
				},
				inlineKeyboardButtons...,
			)
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

// trackerReportInterval is how long a tracker waits between progress reports.
// The first one goes out immediately, so a slow download does not sit silent
// until the interval elapses.
const trackerReportInterval = 5 * time.Minute

// handles track* and untrack* callbacks
func TrackTorrentCallbackHandler(m *media.Media) bot.HandlerFunc {
	mu := sync.RWMutex{}
	cancels := map[string]context.CancelFunc{}

	return func(ctx context.Context, bot *bot.Bot, update *models.Update) {
		sender := &callbackMessageSender{
			messageSender: messageSender{ctx, bot, update},
			keepOnAnswer:  true,
		}

		id, data, err := sender.parseCallback(2)

		if err != nil {
			slog.ErrorContext(ctx, err.Error())
			return
		}

		// empty userID is okay
		userID, _ := ctx.Value("userID").(string)
		callbackName := data[0]
		trackerID := userID + id

		mu.RLock()
		cancel, ok := cancels[trackerID]
		mu.RUnlock()

		if callbackName == "untrack" {
			if ok {
				mu.Lock()
				cancel()
				delete(cancels, trackerID)
				mu.Unlock()
				sender.sendMessage("<i>Stopped sending notifications</i>")
			}
			return
		}
		if ok {
			sender.sendMessage("<i>Notifications for the torrent are already running</i>")
			return
		}

		trackerCtx, cancel := context.WithCancel(ctx)

		// todo: make it block when there's too many notification goroutines running?
		mu.Lock()
		cancels[trackerID] = cancel
		mu.Unlock()

		go func() {
			for {
				torrentStatus, err := m.GetCurrentTorrent(ctx, id)
				if err != nil {
					slog.ErrorContext(ctx, err.Error())
					sender.sendError(err)
					return
				}

				if torrentStatus.Done {
					sender.sendMessage(fmt.Sprintf("✅ Finished downloading: %s", torrentStatus.Name))
					mu.Lock()
					delete(cancels, trackerID)
					mu.Unlock()
					return
				}

				sender.sendMessageWithKeyboard(
					fmt.Sprintf(
						"🔄 %.2f%%\n⌛ ETA %s:<blockquote>%s</blockquote>",
						torrentStatus.PercentDone,
						torrentStatus.ETA,
						torrentStatus.Name,
					),
					[][]models.InlineKeyboardButton{
						{{
							Text:         "⏹️ Stop",
							Style:        styleDanger,
							CallbackData: fmt.Sprintf("untrack:%d", torrentStatus.ID),
						}},
					},
				)

				select {
				case <-time.After(trackerReportInterval):
				case <-trackerCtx.Done():
					return
				}
			}
		}()
	}
}
