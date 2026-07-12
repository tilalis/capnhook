package interfaces

import (
	"context"
	"time"
)

// SearchClient is a generic torrent search client interface
type SearchClient interface {
	Find(ctx context.Context, id string) (TorrentSearchResult, error)
	Search(ctx context.Context, query string, limit int) ([]TorrentSearchResult, error)
	SiteUrl(t TorrentSearchResult) string
}

type TorrentSearchResult interface {
	ID() string
	Name() string
	SizeGB() float64
	NumFiles() int64
	AddedTime() time.Time
	Username() string
	Description() string
	MagnetLink() string
}

// TorrentFileProvider is implemented by search results that can supply the
// raw contents of a .torrent file. Media prefers it over MagnetLink so that
// the torrent client does not have to fetch anything from the network itself.
type TorrentFileProvider interface {
	TorrentFile(ctx context.Context) ([]byte, error)
}
