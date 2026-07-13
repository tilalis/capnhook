package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// utility interface to wrap telegram bot message sending
type messager interface {
	sendMessage(responseText string) error
	sendMessageWithKeyboard(responseText string, keyboard [][]models.InlineKeyboardButton) error
	sendError(sendErr error)
}

type messageSender struct {
	ctx    context.Context
	bot    *bot.Bot
	update *models.Update
}

func (s *messageSender) chatID() (int64, error) {
	var chatID int64

	if s.update.Message != nil {
		chatID = s.update.Message.Chat.ID
	} else if cq := s.update.CallbackQuery; cq != nil && cq.Message.Message != nil {
		chatID = s.update.CallbackQuery.Message.Message.Chat.ID
	} else {
		return 0, errors.New("can't extract chat ID from message")
	}

	return chatID, nil
}

func (s *messageSender) sendMessage(responseText string) error {
	chatID, err := s.chatID()
	if err != nil {
		return err
	}

	_, err = s.bot.SendMessage(s.ctx, &bot.SendMessageParams{
		ChatID:    chatID,
		Text:      responseText,
		ParseMode: models.ParseModeHTML,
	})

	return err
}

func (s *messageSender) sendMessageWithKeyboard(
	responseText string,
	keyboard [][]models.InlineKeyboardButton,
) error {
	chatID, err := s.chatID()
	if err != nil {
		return err
	}

	_, err = s.bot.SendMessage(s.ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   responseText,
		ReplyMarkup: &models.InlineKeyboardMarkup{
			InlineKeyboard: keyboard,
		},
		ParseMode: models.ParseModeHTML,
	})

	return err
}

func (s *messageSender) sendError(sendErr error) {
	chatID, err := s.chatID()
	if err != nil {
		slog.ErrorContext(s.ctx, "can't report error to user", "error", err, "original_error", sendErr)
		return
	}

	if _, err := s.bot.SendMessage(s.ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   fmt.Sprintf("An error happened: %s", sendErr.Error()),
	}); err != nil {
		slog.ErrorContext(s.ctx, "failed to send error message", "error", err, "original_error", sendErr)
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
