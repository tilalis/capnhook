package handlers

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/hekmon/transmissionrpc/v3"
	"github.com/scalalang2/golang-fifo/sieve"
	"github.com/tilalis/capnhook/piratebay"
	"github.com/tilalis/capnhook/transmission"
)

type BotHandlerSet struct {
	piratebay    *piratebay.Piratebay
	transmission *transmission.TransmissionClient
	cache        *sieve.Sieve[string, *piratebay.Torrent]
}

func NewBot() (*bot.Bot, error) {
	handlerSet := NewHandlerSet()
	b, err := bot.New(os.Getenv("TELEGRAM_BOT_TOKEN"), handlerSet.BotOptions()...)
	if err != nil {
		return nil, err
	}
	handlerSet.RegisterHandlers(b)
	return b, nil
}

func NewHandlerSet() *BotHandlerSet {
	transmission, err := transmission.NewTransmissionClient("")

	if err != nil {
		panic(err)
	}

	return &BotHandlerSet{
		piratebay:    piratebay.NewDefault(),
		transmission: transmission,
		cache:        sieve.New[string, *piratebay.Torrent](16, 0),
	}
}

func (bhs *BotHandlerSet) BotOptions() []bot.Option {
	return []bot.Option{
		bot.WithMiddlewares(usersWhitelist),
		bot.WithDefaultHandler(bhs.defaultHandler),
		bot.WithCallbackQueryDataHandler("id", bot.MatchTypePrefix, bhs.showTorrentInfoCallbackHandler),
		bot.WithCallbackQueryDataHandler("download", bot.MatchTypePrefix, bhs.downloadCallbackHandler),
		bot.WithCallbackQueryDataHandler("torrent", bot.MatchTypePrefix, bhs.manageTorrentCallbackHandler),
		bot.WithCallbackQueryDataHandler("deletetorrent", bot.MatchTypePrefix, bhs.deleteTorrentCallbackHandler),
	}
}

func (bhs *BotHandlerSet) RegisterHandlers(b *bot.Bot) {
	b.RegisterHandler(bot.HandlerTypeMessageText, "info", bot.MatchTypeCommandStartOnly, bhs.infoCommandHandler)
	b.RegisterHandler(bot.HandlerTypeMessageText, "inprogress", bot.MatchTypeCommandStartOnly, bhs.infoCommandHandler)
	b.RegisterHandler(bot.HandlerTypeMessageText, "space", bot.MatchTypeCommandStartOnly, bhs.spaceCommandHandler)
}

func (bhs *BotHandlerSet) defaultHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message == nil {
		log.Print("Could not process the update")
		return
	}
	text := update.Message.Text

	torrents, err := bhs.piratebay.Search(text)

	if err != nil {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   fmt.Sprintf("an error happened: %s", err.Error()),
		})
	}

	if len(torrents) > 30 {
		torrents = torrents[:31]
	}

	var responseText strings.Builder
	var inlineKeyboard [][]models.InlineKeyboardButton

	for i, tor := range torrents {
		fmt.Fprintf(
			&responseText,
			"<b>%d</b>: <code>%s</code>\n%.2fGB, %s files, %s by %s\n\n",
			i,
			tor.Name,
			tor.SizeGB(),
			tor.NumFiles,
			tor.AddedTime().Format("2006-01-02"),
			tor.Username,
		)

		inlineKeyboard = append(
			inlineKeyboard,
			[]models.InlineKeyboardButton{{Text: fmt.Sprintf("%d: %s", i, tor.Name), CallbackData: fmt.Sprintf("id:%s", tor.Id)}},
		)
	}

	kb := &models.InlineKeyboardMarkup{
		InlineKeyboard: inlineKeyboard,
	}

	_, err = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      update.Message.Chat.ID,
		Text:        responseText.String(),
		ReplyMarkup: kb,
		ParseMode:   models.ParseModeHTML,
	})

	if err != nil {
		panic(err)
	}
}

func (bhs *BotHandlerSet) spaceCommandHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	freeSpace, _, err := bhs.transmission.FreeSpace(ctx, "/home/tilalis/Plex")

	if err != nil {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.CallbackQuery.Message.Message.Chat.ID,
			Text:   err.Error(),
		})
		return
	}

	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   fmt.Sprintf("💾 Free Space: %s", freeSpace.GiBString()),
	})
}

func (bhs *BotHandlerSet) infoCommandHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	torrents, err := bhs.transmission.TorrentGetAll(ctx)

	if err != nil {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.CallbackQuery.Message.Message.Chat.ID,
			Text:   "Something went wrong :(",
		})
		return
	}

	inProgress := strings.HasPrefix(update.Message.Text, "/inprogress")

	var responseText strings.Builder
	var inlineKeyboard [][]models.InlineKeyboardButton

	for i, torrent := range torrents {
		var status string
		percentDone := *torrent.PercentDone
		if percentDone == 1.0 {
			status = "✅"
			if inProgress {
				continue
			}
		} else {
			status = "🔄"
		}

		var etaHours float64

		if rawEta := *torrent.ETA; rawEta > 0 {
			eta := time.Duration(*torrent.ETA) * time.Second
			etaHours = eta.Hours()
		} else {
			etaHours = 0.0
		}

		name := *torrent.Name
		sizeWhenDone := *torrent.SizeWhenDone

		fmt.Fprintf(
			&responseText,
			"%d: %s %s <code>(%.0f%%, %.2fh, %s)</code>\n",
			i,
			status,
			name,
			percentDone*100.0,
			etaHours,
			sizeWhenDone.GiBString(),
		)

		inlineKeyboard = append(inlineKeyboard, []models.InlineKeyboardButton{
			{Text: fmt.Sprintf("%d: %s", i, name), CallbackData: fmt.Sprintf("torrent:%d", *torrent.ID)},
		})
	}

	var message string
	var replyMarkup models.ReplyMarkup

	if responseText.Len() == 0 {
		message = "<i>No downloads in progress</i>"
		replyMarkup = nil
	} else {
		message = responseText.String()
		replyMarkup = &models.InlineKeyboardMarkup{
			InlineKeyboard: inlineKeyboard,
		}
	}

	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      update.Message.Chat.ID,
		Text:        message,
		ParseMode:   models.ParseModeHTML,
		ReplyMarkup: replyMarkup,
	})
}

func (bhs *BotHandlerSet) deleteTorrentCallbackHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
		CallbackQueryID: update.CallbackQuery.ID,
		ShowAlert:       false,
	})

	b.DeleteMessage(ctx, &bot.DeleteMessageParams{
		ChatID:    update.CallbackQuery.Message.Message.Chat.ID,
		MessageID: update.CallbackQuery.Message.Message.ID,
	})

	if strings.HasPrefix(update.CallbackQuery.Data, "deletetorrentback") {
		update.Message = update.CallbackQuery.Message.Message
		bhs.infoCommandHandler(ctx, b, update)
		return
	}

	data := strings.SplitN(update.CallbackQuery.Data, ":", 2)

	if len(data) < 2 {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.CallbackQuery.Message.Message.Chat.ID,
			Text:   "Something went wrong :(",
		})
		return
	}

	id, err := strconv.Atoi(data[1])

	if err != nil {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.CallbackQuery.Message.Message.Chat.ID,
			Text:   err.Error(),
		})
		return
	}

	torrent, err := bhs.transmission.TorrentGetByID(ctx, int64(id))

	if err != nil {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.CallbackQuery.Message.Message.Chat.ID,
			Text:   err.Error(),
		})
		return
	}

	err = bhs.transmission.TorrentRemove(
		ctx,
		transmissionrpc.TorrentRemovePayload{
			IDs:             []int64{*torrent.ID},
			DeleteLocalData: true,
		},
	)

	if err != nil {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.CallbackQuery.Message.Message.Chat.ID,
			Text:   err.Error(),
		})
		return
	}

	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    update.CallbackQuery.Message.Message.Chat.ID,
		Text:      fmt.Sprintf("🗑️ Deleted with all files: %s", *torrent.Name),
		ParseMode: models.ParseModeHTML,
	})
}

func (bhs *BotHandlerSet) manageTorrentCallbackHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
		CallbackQueryID: update.CallbackQuery.ID,
		ShowAlert:       false,
	})

	b.DeleteMessage(ctx, &bot.DeleteMessageParams{
		ChatID:    update.CallbackQuery.Message.Message.Chat.ID,
		MessageID: update.CallbackQuery.Message.Message.ID,
	})

	data := strings.SplitN(update.CallbackQuery.Data, ":", 2)

	if len(data) < 2 {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.CallbackQuery.Message.Message.Chat.ID,
			Text:   "Something went wrong :(",
		})
		return
	}

	id, err := strconv.Atoi(data[1])

	if err != nil {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.CallbackQuery.Message.Message.Chat.ID,
			Text:   err.Error(),
		})
		return
	}

	torrent, err := bhs.transmission.TorrentGetByID(ctx, int64(id))

	if err != nil {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.CallbackQuery.Message.Message.Chat.ID,
			Text:   err.Error(),
		})
		return
	}

	sizeWhenDone := *torrent.SizeWhenDone

	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    update.CallbackQuery.Message.Message.Chat.ID,
		Text:      fmt.Sprintf("%s <code>(%.0f%%, %s)</code>", *torrent.Name, *torrent.PercentDone*100.0, sizeWhenDone.GiBString()),
		ParseMode: models.ParseModeHTML,
		ReplyMarkup: &models.InlineKeyboardMarkup{
			InlineKeyboard: [][]models.InlineKeyboardButton{
				{{Text: "🚫 Delete with all files", CallbackData: fmt.Sprintf("deletetorrent:%d", *torrent.ID)}},
				{{Text: "⬅️ Back", CallbackData: "deletetorrentback"}},
			},
		},
	})
}

func (bhs *BotHandlerSet) showTorrentInfoCallbackHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	// answering callback query first to let Telegram know that we received the callback query,
	b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
		CallbackQueryID: update.CallbackQuery.ID,
		ShowAlert:       false,
	})

	data := strings.SplitN(update.CallbackQuery.Data, ":", 2)

	if len(data) < 2 {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.CallbackQuery.Message.Message.Chat.ID,
			Text:   "Something went wrong :(",
		})
	}

	id := data[1]
	torrent, err := bhs.findTorrent(id)

	if err != nil {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.CallbackQuery.Message.Message.Chat.ID,
			Text:   err.Error(),
		})
		return
	}

	b.SendMessage(
		ctx,
		&bot.SendMessageParams{
			ChatID: update.CallbackQuery.Message.Message.Chat.ID,
			Text: fmt.Sprintf(
				"<a href=\"%s\">%s</a>\n<code>%.2fGB | %s files | %s | by %s</code>\n\n<blockquote>%s</blockquote>",
				bhs.piratebay.SiteUrl(torrent),
				torrent.Name,
				torrent.SizeGB(),
				torrent.NumFiles,
				torrent.AddedTime().Format("2006-01-02"),
				torrent.Username,
				torrent.Descr,
			),
			ParseMode: models.ParseModeHTML,
			ReplyMarkup: &models.InlineKeyboardMarkup{
				InlineKeyboard: [][]models.InlineKeyboardButton{
					{{Text: "📺 Download to Movies", CallbackData: fmt.Sprintf("download:movies:%s", torrent.Id)}},
					{{Text: "🎬 Download to TVShows", CallbackData: fmt.Sprintf("download:tvshows:%s", torrent.Id)}},
				},
			},
		},
	)
}

func (bhs *BotHandlerSet) downloadCallbackHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
		CallbackQueryID: update.CallbackQuery.ID,
		ShowAlert:       false,
	})

	data := strings.SplitN(update.CallbackQuery.Data, ":", 3)

	if len(data) < 3 {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.CallbackQuery.Message.Message.Chat.ID,
			Text:   "Something went wrong :(",
		})
	}

	dest, id := data[1], data[2]

	var downloadDir string

	switch dest {
	case "movies":
		downloadDir = "/home/tilalis/Plex/Movies/"
	case "tvshows":
		downloadDir = "/home/tilalis/Plex/TVShows/"
	}

	torrent, err := bhs.findTorrent(id)

	if err != nil {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.CallbackQuery.Message.Message.Chat.ID,
			Text:   err.Error(),
		})
		return
	}

	magnet := torrent.MagnetLink()

	_, err = bhs.transmission.TorrentAdd(ctx, transmissionrpc.TorrentAddPayload{
		Filename:    &magnet,
		DownloadDir: &downloadDir,
	})

	if err != nil {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.CallbackQuery.Message.Message.Chat.ID,
			Text:   err.Error(),
		})
		return
	}

	b.SendMessage(
		ctx,
		&bot.SendMessageParams{
			ChatID:    update.CallbackQuery.Message.Message.Chat.ID,
			Text:      fmt.Sprintf("📥 <i>Downloading %s to %s</i>", torrent.Name, downloadDir),
			ParseMode: models.ParseModeHTML,
		},
	)
}

func (bhs *BotHandlerSet) findTorrent(id string) (torrent *piratebay.Torrent, err error) {
	torrent, ok := bhs.cache.Get(id)

	if !ok {
		torrent, err = bhs.piratebay.Find(id)
		if err != nil {
			return
		}
		bhs.cache.Set(id, torrent)
	}

	return
}
