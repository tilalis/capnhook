package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/go-telegram/bot"
)

const (
	defaultWebhookAddr = ":8080"

	// secretTokenHeader is the header Telegram echoes the configured secret in.
	secretTokenHeader = "X-Telegram-Bot-Api-Secret-Token"

	webhookReadHeaderTimeout = 10 * time.Second
	webhookShutdownTimeout   = 10 * time.Second
)

// webhookConfig describes how the bot is reachable when Telegram pushes updates
// to it instead of the bot polling for them.
type webhookConfig struct {
	url         string
	listenAddr  string
	path        string
	secretToken string
}

// parseWebhookConfig reads the webhook settings from the environment. A nil
// config (with a nil error) means TELEGRAM_WEBHOOK_URL is unset and the bot
// should fall back to long polling.
func parseWebhookConfig() (*webhookConfig, error) {
	rawURL := strings.TrimSpace(os.Getenv("TELEGRAM_WEBHOOK_URL"))
	if rawURL == "" {
		return nil, nil
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid TELEGRAM_WEBHOOK_URL %q: %w", rawURL, err)
	}

	// Telegram only delivers updates over HTTPS.
	if parsed.Scheme != "https" || parsed.Host == "" {
		return nil, fmt.Errorf("TELEGRAM_WEBHOOK_URL must be an https url, got %q", rawURL)
	}

	// Updates arrive at the path of the public url, so serve that same path —
	// unless a proxy in front of the bot rewrites it and says otherwise.
	path := strings.TrimSpace(os.Getenv("TELEGRAM_WEBHOOK_PATH"))
	if path == "" {
		path = parsed.Path
	}
	if path == "" {
		path = "/"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	listenAddr := strings.TrimSpace(os.Getenv("TELEGRAM_WEBHOOK_ADDR"))
	if listenAddr == "" {
		listenAddr = defaultWebhookAddr
	}

	return &webhookConfig{
		url:         rawURL,
		listenAddr:  listenAddr,
		path:        path,
		secretToken: strings.TrimSpace(os.Getenv("TELEGRAM_WEBHOOK_SECRET")),
	}, nil
}

// servePolling pulls updates with long polling and blocks until ctx is done.
// Any webhook left over from a previous run is removed first, as Telegram
// refuses getUpdates while one is registered.
func servePolling(ctx context.Context, b *bot.Bot) error {
	// Nil params on purpose: an all-default DeleteWebhookParams has every field
	// omitempty, so it marshals to a multipart body with no fields at all, which
	// Telegram answers with an empty 400 the client cannot decode.
	if _, err := b.DeleteWebhook(ctx, nil); err != nil {
		return fmt.Errorf("could not remove a previously registered webhook: %w", err)
	}

	slog.InfoContext(ctx, "Serving Telegram updates with long polling")
	b.Start(ctx)

	return nil
}

// serveWebhook registers the webhook with Telegram, serves the updates it
// pushes over HTTP and blocks until ctx is done or the server fails. The
// webhook is deliberately left registered on shutdown so that Telegram queues
// updates and redelivers them once the bot is back.
func serveWebhook(ctx context.Context, b *bot.Bot, config *webhookConfig) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	mux := http.NewServeMux()
	mux.Handle("POST "+config.path, webhookHandler(b, config.secretToken))

	server := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: webhookReadHeaderTimeout,
	}
	defer server.Close()

	listener, err := net.Listen("tcp", config.listenAddr)
	if err != nil {
		return fmt.Errorf("could not listen on %s: %w", config.listenAddr, err)
	}

	serveErr := make(chan error, 1)
	go func() {
		// Unblock the bot workers below when the server stops on its own.
		defer cancel()

		err := server.Serve(listener)
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}

		serveErr <- err
	}()

	// Registered only once the endpoint accepts connections, so that the very
	// first update Telegram pushes is not dropped.
	if _, err := b.SetWebhook(ctx, &bot.SetWebhookParams{
		URL:         config.url,
		SecretToken: config.secretToken,
	}); err != nil {
		return errors.Join(fmt.Errorf("could not register the webhook: %w", err), drain(serveErr))
	}

	slog.InfoContext(
		ctx, "Serving Telegram updates over a webhook",
		"url", config.url, "address", config.listenAddr, "path", config.path,
	)
	b.StartWebhook(ctx)

	shutdownCtx, stopShutdown := context.WithTimeout(context.WithoutCancel(ctx), webhookShutdownTimeout)
	defer stopShutdown()

	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("could not shut the webhook server down: %w", err)
	}

	return <-serveErr
}

// webhookHandler checks the secret token before handing the update over to the
// bot. bot.WebhookHandler does check it too, but it answers a rejected update
// with 200 — it drops the ResponseWriter on the floor — and Telegram reads that
// as a successful delivery. A webhook that silently discards every update then
// looks exactly like a healthy one in getWebhookInfo: nothing pending, no last
// error. Answering 403 puts the failure where it can be seen.
func webhookHandler(b *bot.Bot, secretToken string) http.Handler {
	deliver := b.WebhookHandler()

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if secretToken != "" && r.Header.Get(secretTokenHeader) != secretToken {
			slog.WarnContext(
				r.Context(),
				"Rejected a webhook request carrying a wrong secret token",
				"remote", r.RemoteAddr,
			)
			http.Error(w, "forbidden", http.StatusForbidden)

			return
		}

		deliver(w, r)
	})
}

// drain returns the error already sent to errs, if any, without waiting for one.
func drain(errs <-chan error) error {
	select {
	case err := <-errs:
		return err
	default:
		return nil
	}
}
