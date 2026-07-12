package internetarchive

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Torrent is a single Internet Archive item exposed as a torrent search result.
type Torrent struct {
	identifier  string
	title       string
	description string
	creator     string
	sizeBytes   int64
	numFiles    int64
	added       time.Time
	torrentUrl  string
}

func (t Torrent) ID() string {
	return t.identifier
}

func (t Torrent) Name() string {
	if t.title == "" {
		return t.identifier
	}
	return t.title
}

func (t Torrent) SizeGB() float64 {
	return float64(t.sizeBytes) / float64(1<<30)
}

func (t Torrent) NumFiles() int64 {
	return t.numFiles
}

func (t Torrent) AddedTime() time.Time {
	return t.added
}

func (t Torrent) Username() string {
	return t.creator
}

func (t Torrent) Description() string {
	return t.description
}

// MagnetLink returns the URL of the item's archive torrent file. It is a
// fallback only: Media prefers TorrentFile, which hands the torrent contents
// to Transmission directly.
func (t Torrent) MagnetLink() string {
	return t.torrentUrl
}

// TorrentFile downloads the item's archive torrent file so its contents can
// be handed to Transmission directly, sparing the daemon from needing
// network access to archive.org.
func (t Torrent) TorrentFile(ctx context.Context) ([]byte, error) {
	return get(ctx, t.torrentUrl)
}

// searchResponse mirrors the advanced search API envelope.
type searchResponse struct {
	Response struct {
		Docs []searchDoc `json:"docs"`
	} `json:"response"`
}

type searchDoc struct {
	Identifier  string     `json:"identifier"`
	Title       flexString `json:"title"`
	Description flexString `json:"description"`
	Creator     flexString `json:"creator"`
	ItemSize    int64      `json:"item_size"`
	FilesCount  int64      `json:"files_count"`
	PublicDate  flexString `json:"publicdate"`
}

// metadataResponse mirrors the metadata API envelope.
type metadataResponse struct {
	ItemSize   int64        `json:"item_size"`
	FilesCount int64        `json:"files_count"`
	Metadata   itemMetadata `json:"metadata"`
}

type itemMetadata struct {
	Identifier  string     `json:"identifier"`
	Mediatype   flexString `json:"mediatype"`
	Title       flexString `json:"title"`
	Description flexString `json:"description"`
	Creator     flexString `json:"creator"`
	LicenseUrl  flexString `json:"licenseurl"`
	PublicDate  flexString `json:"publicdate"`
}

// flexString absorbs Internet Archive metadata values that may be either a
// single string or an array of strings.
type flexString string

func (f *flexString) UnmarshalJSON(data []byte) error {
	var single string
	if err := json.Unmarshal(data, &single); err == nil {
		*f = flexString(single)
		return nil
	}

	var many []string
	if err := json.Unmarshal(data, &many); err != nil {
		return fmt.Errorf("value is neither a string nor an array of strings: %s", data)
	}

	*f = flexString(strings.Join(many, "\n"))
	return nil
}
