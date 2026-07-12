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

