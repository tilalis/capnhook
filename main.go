package main

import (
	"context"
	"log"
	"os"
	"os/signal"

	"github.com/go-telegram/bot"
	"github.com/joho/godotenv"
	"github.com/tilalis/capnhook/handlers"
)

// Send any text message to the bot after the bot has been started

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Could not read .env")
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	b, err := bot.New(os.Getenv("TELEGRAM_BOT_TOKEN"), handlers.New().BotOptions()...)
	if err != nil {
		panic(err)
	}

	b.Start(ctx)

	// client, err := transmission.NewTransmissionClient("")

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

}

