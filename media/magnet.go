package media

import (
	"encoding/base32"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// magnetScheme prefixes every magnet URI.
const magnetScheme = "magnet:"

// btihPrefix marks the one xt topic we can act on: a BitTorrent v1 info hash.
// Links may carry other topics (btmh for v2, ed2k, sha1) and we skip those.
const btihPrefix = "urn:btih:"

var (
	errNotAMagnet = errors.New("not a magnet link")
	errNoInfoHash = errors.New("magnet link carries no btih info hash")
)

// Magnet is a magnet link the bot was asked to download.
type Magnet struct {
	// InfoHash is the torrent's BTIH as 40 lowercase hex characters. Callback
	// data is capped at 64 bytes by Telegram and a magnet URI easily runs to
	// several hundred, so this is the part that travels in a button.
	InfoHash string

	// DisplayName is the dn parameter, empty when the link carries none.
	DisplayName string

	// URI is the link as received, trackers and all. Transmission is handed
	// this rather than a hash rebuilt from InfoHash, so the tracker list and
	// any web seeds survive.
	URI string
}

// IsMagnet reports whether text is a magnet link.
func IsMagnet(text string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(text)), magnetScheme)
}

// ParseMagnet pulls the info hash and display name out of a magnet link.
func ParseMagnet(raw string) (Magnet, error) {
	raw = strings.TrimSpace(raw)

	if !IsMagnet(raw) {
		return Magnet{}, errNotAMagnet
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		return Magnet{}, fmt.Errorf("parse magnet link: %w", err)
	}

	query := parsed.Query()

	for _, topic := range query["xt"] {
		infoHash, ok := infoHashFromTopic(topic)
		if !ok {
			continue
		}

		return Magnet{
			InfoHash:    infoHash,
			DisplayName: strings.TrimSpace(query.Get("dn")),
			URI:         raw,
		}, nil
	}

	return Magnet{}, errNoInfoHash
}

// infoHashFromTopic normalizes an xt topic into a lowercase hex info hash.
// Magnet links spell the hash either as 40 hex characters or as 32 base32
// ones. Transmission accepts both, but only one spelling can be cached and
// compared, so base32 is folded into hex here.
func infoHashFromTopic(topic string) (string, bool) {
	if !strings.HasPrefix(strings.ToLower(topic), btihPrefix) {
		return "", false
	}

	hash := topic[len(btihPrefix):]

	switch len(hash) {
	case 40:
		if _, err := hex.DecodeString(hash); err != nil {
			return "", false
		}

		return strings.ToLower(hash), true
	case 32:
		decoded, err := base32.StdEncoding.DecodeString(strings.ToUpper(hash))
		if err != nil {
			return "", false
		}

		return hex.EncodeToString(decoded), true
	default:
		return "", false
	}
}
