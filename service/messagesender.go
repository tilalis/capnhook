package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

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
		return
	}

	s.bot.SendMessage(s.ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   fmt.Sprintf("An error happened: %s", sendErr.Error()),
	})
}
