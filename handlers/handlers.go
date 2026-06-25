package handlers

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/hekmon/transmissionrpc/v3"
	"github.com/scalalang2/golang-fifo/sieve"
	"github.com/tilalis/capnhook/piratebay"
	"github.com/tilalis/capnhook/transmission"
)

type BotHandlerSet struct {
	piratebay *piratebay.Piratebay
	transmission *transmission.TransmissionClient
	cache     *sieve.Sieve[string, *piratebay.Torrent]
}

func New() *BotHandlerSet {
	transmission, err := transmission.NewTransmissionClient("")

	if err != nil {
		panic(err)
	}

	return &BotHandlerSet{
		piratebay: piratebay.NewDefault(),
		transmission: transmission,
		cache:     sieve.New[string, *piratebay.Torrent](16, 0),
	}
}

func (bhs *BotHandlerSet) BotOptions() []bot.Option {
	return []bot.Option{
		bot.WithDefaultHandler(bhs.defaultHandler),
		bot.WithCallbackQueryDataHandler("id", bot.MatchTypePrefix, bhs.selectTorrentHandler),
		bot.WithCallbackQueryDataHandler("download", bot.MatchTypePrefix, bhs.downloadCallbackHandler),
	}
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

func (bhs *BotHandlerSet) selectTorrentHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
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
		Filename: &magnet,
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
			ChatID: update.CallbackQuery.Message.Message.Chat.ID,
			Text: fmt.Sprintf("📥 <i>Downloading %s to %s</i>", torrent.Name, downloadDir),
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
