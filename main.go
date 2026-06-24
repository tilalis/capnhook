package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/tilalis/capnhook/piratebay"
)

// Send any text message to the bot after the bot has been started

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	// client, err := torrents.NewTransmissionClient("")

	// if err != nil {
	// 	panic(err)
	// }

	// magnet := "magnet:?xt=urn:btih:BEC9F646FC9A310CC54D2EF8C32F19607C11C80D&dn=Citizen%20Kane%20(1941)%20720p%20BRRiP%20x264%20AAC%20%5BTeam%20Nanban%5D&tr=udp%3A%2F%2Ftracker.opentrackr.org%3A1337&tr=udp%3A%2F%2Fopen.stealth.si%3A80%2Fannounce&tr=udp%3A%2F%2Ftracker.torrent.eu.org%3A451%2Fannounce&tr=udp%3A%2F%2Ftracker.bittor.pw%3A1337%2Fannounce&tr=udp%3A%2F%2Fpublic.popcorn-tracker.org%3A6969%2Fannounce&tr=udp%3A%2F%2Ftracker.dler.org%3A6969%2Fannounce&tr=udp%3A%2F%2Fexodus.desync.com%3A6969&tr=udp%3A%2F%2Fopen.demonii.com%3A1337%2Fannounce&tr=udp%3A%2F%2Fglotorrents.pw%3A6969%2Fannounce&tr=udp%3A%2F%2Ftracker.coppersurfer.tk%3A6969&tr=udp%3A%2F%2Ftorrent.gresille.org%3A80%2Fannounce&tr=udp%3A%2F%2Fp4p.arenabg.com%3A1337&tr=udp%3A%2F%2Ftracker.internetwarriors.net%3A1337"

	// torrent, err := client.TorrentAdd(ctx, transmissionrpc.TorrentAddPayload{
	// 	Filename: &magnet,
	// })

	// if err != nil {
	// 	panic(err)
	// }

	// fmt.Println(torrent.IsFinished)

	// time.Sleep(time.Second * 2)

	// torrent, err = client.TorrentGetByID(ctx, *torrent.ID)

	// fmt.Println(torrent.IsFinished)

	opts := []bot.Option{
		bot.WithNotAsyncHandlers(),
		bot.WithDefaultHandler(handler),
		bot.WithCallbackQueryDataHandler("id", bot.MatchTypePrefix, callbackHandler),
	}

	b, err := bot.New("1347708974:AAGylv4Sy_upeVmqUwIZO_Q-_h67xkbaXnQ", opts...)
	if err != nil {
		panic(err)
	}

	b.Start(ctx)
}

func handler(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message == nil {
		log.Print("No message")
		return
	}
	text := update.Message.Text

	pb := piratebay.New("")
	torrents, err := pb.Search(text)

	if err != nil {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   fmt.Sprintf("an error happened: %s", err.Error()),
		})
	}

	var responseText strings.Builder
	var inlineKeyboard [][]models.InlineKeyboardButton

	for _, tor := range torrents {
		fmt.Fprintf(&responseText, "`%s`\n%.2fGB, %s files, %s by %s\n\n", tor.Name, tor.SizeGB(), tor.NumFiles, tor.AddedTime().Format("2006-01-02"), tor.Username)
		inlineKeyboard = append(inlineKeyboard, []models.InlineKeyboardButton{{Text: tor.Name, CallbackData: "id:" + tor.Id}})
	}

	kb := &models.InlineKeyboardMarkup{
		InlineKeyboard: inlineKeyboard,
	}

	_, err = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   responseText.String(),
		ReplyMarkup: kb,
	})

	if err != nil {
		panic(err)
	}
}

func callbackHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	// answering callback query first to let Telegram know that we received the callback query,
	// and we're handling it. Otherwise, Telegram might retry sending the update repetitively
	// as it thinks the callback query doesn't reach to our application. learn more by
	// reading the footnote of the https://core.telegram.org/bots/api#callbackquery type.
	b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
		CallbackQueryID: update.CallbackQuery.ID,
		ShowAlert:       false,
	})
	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.CallbackQuery.Message.Message.Chat.ID,
		Text:   "You selected the button: " + update.CallbackQuery.Data,
	})
}
