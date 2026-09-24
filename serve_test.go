package main

import "testing"

func TestParseWebhookConfig(t *testing.T) {
	testCases := []struct {
		name     string
		env      map[string]string
		expected *webhookConfig
		fails    bool
	}{
		{
			name:     "no url falls back to polling",
			env:      map[string]string{"TELEGRAM_WEBHOOK_URL": ""},
			expected: nil,
		},
		{
			name: "path and defaults are derived from the url",
			env:  map[string]string{"TELEGRAM_WEBHOOK_URL": "https://bot.example.com/telegram/hook"},
			expected: &webhookConfig{
				url:        "https://bot.example.com/telegram/hook",
				listenAddr: defaultWebhookAddr,
				path:       "/telegram/hook",
			},
		},
		{
			name: "a url without a path serves the root",
			env:  map[string]string{"TELEGRAM_WEBHOOK_URL": "https://bot.example.com"},
			expected: &webhookConfig{
				url:        "https://bot.example.com",
				listenAddr: defaultWebhookAddr,
				path:       "/",
			},
		},
		{
			name: "explicit settings win over the derived ones",
			env: map[string]string{
				"TELEGRAM_WEBHOOK_URL":    "https://bot.example.com/public",
				"TELEGRAM_WEBHOOK_PATH":   "internal",
				"TELEGRAM_WEBHOOK_ADDR":   "127.0.0.1:9000",
				"TELEGRAM_WEBHOOK_SECRET": "s3cret",
			},
			expected: &webhookConfig{
				url:         "https://bot.example.com/public",
				listenAddr:  "127.0.0.1:9000",
				path:        "/internal",
				secretToken: "s3cret",
			},
		},
		{
			name:  "a plain http url is rejected",
			env:   map[string]string{"TELEGRAM_WEBHOOK_URL": "http://bot.example.com/hook"},
			fails: true,
		},
		{
			name:  "a url without a host is rejected",
			env:   map[string]string{"TELEGRAM_WEBHOOK_URL": "/hook"},
			fails: true,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			for _, key := range []string{
				"TELEGRAM_WEBHOOK_URL",
				"TELEGRAM_WEBHOOK_PATH",
				"TELEGRAM_WEBHOOK_ADDR",
				"TELEGRAM_WEBHOOK_SECRET",
			} {
				t.Setenv(key, testCase.env[key])
			}

			config, err := parseWebhookConfig()

			if testCase.fails {
				if err == nil {
					t.Fatalf("expected an error, got config %+v", config)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if testCase.expected == nil {
				if config != nil {
					t.Fatalf("expected no config, got %+v", config)
				}
				return
			}

			if config == nil || *config != *testCase.expected {
				t.Errorf("got %+v, want %+v", config, testCase.expected)
			}
		})
	}
}
