package service

import (
	"fmt"
	"strings"

	"github.com/go-telegram/bot/models"
	"github.com/tilalis/capnhook/media/interfaces"
	"github.com/tilalis/capnhook/service/paginator"
)

// Inline keyboard button styles, added in Bot API 9.4. Telegram clients older
// than February 9, 2026 ignore the field and draw the button in the default
// color, so a style is decoration only — never the sole signal of what a button
// does.
const (
	stylePrimary = "primary" // blue
	styleSuccess = "success" // green
	styleDanger  = "danger"  // red
)

func sendTorrentsMessage(
	sender messager,
	torrentsPaginator *paginator.Paginator[[]interfaces.TorrentSearchResult, interfaces.TorrentSearchResult],
	query string,
) error {
	var (
		responseText     strings.Builder
		responseKeyboard = make([][]models.InlineKeyboardButton, 0, torrentsPaginator.PageSize()+2)
	)

	for i, torrent := range torrentsPaginator.IterCurrentPage() {
		idx := i + 1
		fmt.Fprintf(
			&responseText,
			"<blockquote><i>Title:</i> %s\n<i>Added:</i> %s\n<i>Size:</i> %.2fGB\n\n%d files, by %s, #%d</blockquote>",
			torrent.Name(),
			torrent.AddedTime().Format("January 2, 2006"),
			torrent.SizeGB(),
			torrent.NumFiles(),
			torrent.Username(),
			idx,
		)

		responseKeyboard = append(
			responseKeyboard,
			[]models.InlineKeyboardButton{
				{Text: fmt.Sprintf("%d: %s", idx, torrent.Name()), CallbackData: fmt.Sprintf("id:%s:%s", torrent.ID(), query)},
			},
		)
	}

	resultsLen, pageNumber, pageSize := torrentsPaginator.Size(), torrentsPaginator.PageNumber(), torrentsPaginator.PageSize()

	// Rounded up, so a trailing short page is still counted.
	totalPages := (resultsLen + pageSize - 1) / pageSize

	fmt.Fprintf(
		&responseText,
		"\n<i>Found %d results. Page %d/%d</i>",
		resultsLen,
		pageNumber+1,
		totalPages,
	)

	if torrentsPaginator.HasNext() {
		responseKeyboard = append(
			responseKeyboard,
			[]models.InlineKeyboardButton{
				{
					Text:         "⏩ Next",
					Style:        stylePrimary,
					CallbackData: fmt.Sprintf("page:%d:%s", torrentsPaginator.PageNumber()+1, query),
				},
			},
		)
	}

	if torrentsPaginator.HasPrev() {
		responseKeyboard = append(
			responseKeyboard,
			[]models.InlineKeyboardButton{
				{
					Text:         "⏪ Previous",
					Style:        stylePrimary,
					CallbackData: fmt.Sprintf("page:%d:%s", torrentsPaginator.PageNumber()-1, query),
				},
			},
		)
	}

	return sender.sendMessageWithKeyboard(responseText.String(), responseKeyboard)
}
