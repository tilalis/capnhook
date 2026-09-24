package media

import (
	"errors"
	"testing"
)

func TestParseMagnet(t *testing.T) {
	// The same info hash spelled both ways magnet links allow.
	const (
		hexHash    = "0123456789abcdef0123456789abcdef01234567"
		base32Hash = "AERUKZ4JVPG66AJDIVTYTK6N54ASGRLH"
	)

	tests := []struct {
		name     string
		raw      string
		wantHash string
		wantName string
		wantErr  error
	}{
		{
			name:     "hex hash with a display name and trackers",
			raw:      "magnet:?xt=urn:btih:" + hexHash + "&dn=Big+Buck+Bunny&tr=udp%3A%2F%2Ftracker.example%3A1337",
			wantHash: hexHash,
			wantName: "Big Buck Bunny",
		},
		{
			name:     "uppercase hex hash is folded to lowercase",
			raw:      "magnet:?xt=urn:btih:0123456789ABCDEF0123456789ABCDEF01234567",
			wantHash: hexHash,
		},
		{
			name:     "base32 hash is converted to hex",
			raw:      "magnet:?xt=urn:btih:" + base32Hash,
			wantHash: hexHash,
		},
		{
			name:     "btih is picked out from among other topics",
			raw:      "magnet:?xt=urn:ed2k:31d6cfe0d16ae931b73c59d7e0c089c0&xt=urn:btih:" + hexHash,
			wantHash: hexHash,
		},
		{
			name:     "surrounding whitespace is ignored",
			raw:      "  magnet:?xt=urn:btih:" + hexHash + "  ",
			wantHash: hexHash,
		},
		{
			name:    "plain text is not a magnet link",
			raw:     "big buck bunny",
			wantErr: errNotAMagnet,
		},
		{
			name:    "magnet without a btih topic",
			raw:     "magnet:?dn=Big+Buck+Bunny&xt=urn:btmh:1220caf1e1",
			wantErr: errNoInfoHash,
		},
		{
			name:    "btih that is neither 40 hex nor 32 base32 characters",
			raw:     "magnet:?xt=urn:btih:abcdef",
			wantErr: errNoInfoHash,
		},
		{
			name:    "btih of the right length that is not hex",
			raw:     "magnet:?xt=urn:btih:zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz",
			wantErr: errNoInfoHash,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			magnet, err := ParseMagnet(test.raw)

			if test.wantErr != nil {
				if !errors.Is(err, test.wantErr) {
					t.Fatalf("ParseMagnet(%q) error = %v, want %v", test.raw, err, test.wantErr)
				}

				return
			}

			if err != nil {
				t.Fatalf("ParseMagnet(%q) returned an unexpected error: %v", test.raw, err)
			}

			if magnet.InfoHash != test.wantHash {
				t.Errorf("InfoHash = %q, want %q", magnet.InfoHash, test.wantHash)
			}

			if magnet.DisplayName != test.wantName {
				t.Errorf("DisplayName = %q, want %q", magnet.DisplayName, test.wantName)
			}

			// Transmission is handed the URI, so it has to survive parsing with
			// its trackers intact.
			if magnet.URI == "" {
				t.Error("URI is empty, Transmission would get nothing to add")
			}
		})
	}
}
