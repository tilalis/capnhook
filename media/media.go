package media

import (
	"context"
	"errors"
	"fmt"
	"path"
	"strconv"
	"time"

	"github.com/hekmon/cunits/v2"
	"github.com/hekmon/transmissionrpc/v3"
	"github.com/scalalang2/golang-fifo/sieve"
	"github.com/tilalis/capnhook/media/interfaces"
	"github.com/tilalis/capnhook/media/clients/apibay"
	"github.com/tilalis/capnhook/media/transmission"
)

type Media struct {
	searchClient                   interfaces.SearchClient
	transmission                   transmissionClient
	cache                          *sieve.Sieve[string, interfaces.TorrentSearchResult]
	rootDir, moviesDir, tvshowsDir string
	maxSearchResults               int
}

// trasmissionClient is the subset of the Transmission RPC client that Media depends on.
type transmissionClient interface {
	TorrentGetByID(ctx context.Context, id int64) (transmissionrpc.Torrent, error)
	TorrentGetAll(ctx context.Context) ([]transmissionrpc.Torrent, error)
	TorrentAdd(ctx context.Context, payload transmissionrpc.TorrentAddPayload) (transmissionrpc.Torrent, error)
	TorrentRemove(ctx context.Context, payload transmissionrpc.TorrentRemovePayload) error
	FreeSpace(ctx context.Context, path string) (freeSpace, totalSize cunits.Bits, err error)
}

type TorrentStatus struct {
	ID           int64
	Name         string
	PercentDone  float64
	ETA          time.Duration
	SizeWhenDone cunits.Bits
	Done         bool
}

func New(root string, s interfaces.SearchClient, tr transmissionClient, maxSearchResults int) *Media {
	if maxSearchResults <= 0 {
		maxSearchResults = defaultMaxSearchResults
	}

	return &Media{
		searchClient:     s,
		transmission:     tr,
		cache:            sieve.New[string, interfaces.TorrentSearchResult](16, 0),
		rootDir:          root,
		moviesDir:        path.Join(root, "Movies"),
		tvshowsDir:       path.Join(root, "TVShows"),
		maxSearchResults: maxSearchResults,
	}
}

// MaxSearchResults returns the configured cap on the number of search results.
func (m *Media) MaxSearchResults() int {
	return m.maxSearchResults
}

// NewDefault builds a Media backed by the default Piratebay client and a
// Transmission client at transmissionURL. rootDir and transmissionURL are
// required; a non-positive maxSearchResults falls back to the built-in default.
func NewDefault(rootDir, transmissionURL string, maxSearchResults int) (*Media, error) {
	tr, err := transmission.NewTransmissionClient(transmissionURL)
	if err != nil {
		return nil, err
	}

	if rootDir == "" {
		return nil, errors.New("rootDir is not specified")
	}

	if transmissionURL == "" {
		return nil, errors.New("transmissionURL is not specified")
	}

	return New(rootDir, apibay.NewDefault(), tr, maxSearchResults), nil
}

var errBadTransmissionRPCResponse = errors.New("bad Transmission RPC response")

// defaultMaxSearchResults caps how many search results are returned when no
// explicit limit is configured.
const defaultMaxSearchResults = 30

func (m *Media) DeleteCurrentTorrent(ctx context.Context, id string) (string, error) {
	identifier, err := strconv.Atoi(id)
	if err != nil {
		return "", fmt.Errorf("invalid torrent id %q: %w", id, err)
	}

	torrent, err := m.transmission.TorrentGetByID(ctx, int64(identifier))
	if err != nil {
		return "", fmt.Errorf("get torrent %d: %w", identifier, err)
	}

	if torrent.ID == nil || torrent.Name == nil {
		return "", errBadTransmissionRPCResponse
	}

	err = m.transmission.TorrentRemove(
		ctx,
		transmissionrpc.TorrentRemovePayload{
			IDs:             []int64{*torrent.ID},
			DeleteLocalData: true,
		},
	)
	if err != nil {
		return "", fmt.Errorf("remove torrent %d: %w", *torrent.ID, err)
	}

	return *torrent.Name, nil
}

// toTorrentStatus maps a raw Transmission torrent into a TorrentStatus,
// validating that every field we rely on is present.
func toTorrentStatus(torrent transmissionrpc.Torrent) (TorrentStatus, error) {
	if torrent.ID == nil || torrent.Name == nil || torrent.SizeWhenDone == nil || torrent.PercentDone == nil || torrent.ETA == nil {
		return TorrentStatus{}, errBadTransmissionRPCResponse
	}

	var eta time.Duration
	if rawEta := *torrent.ETA; rawEta > 0 {
		eta = time.Duration(rawEta) * time.Second
	}

	return TorrentStatus{
		ID:           *torrent.ID,
		Name:         *torrent.Name,
		PercentDone:  *torrent.PercentDone * 100,
		SizeWhenDone: *torrent.SizeWhenDone,
		ETA:          eta,
		Done:         *torrent.PercentDone >= 1.0,
	}, nil
}

func (m *Media) GetCurrentTorrent(ctx context.Context, id string) (*TorrentStatus, error) {
	identifier, err := strconv.Atoi(id)
	if err != nil {
		return nil, fmt.Errorf("invalid torrent id %q: %w", id, err)
	}

	torrent, err := m.transmission.TorrentGetByID(ctx, int64(identifier))
	if err != nil {
		return nil, fmt.Errorf("get torrent %d: %w", identifier, err)
	}

	status, err := toTorrentStatus(torrent)
	if err != nil {
		return nil, err
	}

	return &status, nil
}

func (m *Media) GetCurrentTorrents(ctx context.Context, inProgress bool) ([]TorrentStatus, error) {
	torrents, err := m.transmission.TorrentGetAll(ctx)

	if err != nil {
		return nil, fmt.Errorf("get all torrents: %w", err)
	}

	torrentStatuses := make([]TorrentStatus, 0, len(torrents))

	for _, torrent := range torrents {
		status, err := toTorrentStatus(torrent)
		if err != nil {
			return nil, err
		}

		if inProgress && status.Done {
			continue
		}

		torrentStatuses = append(torrentStatuses, status)
	}

	return torrentStatuses, nil
}

func (m *Media) DownloadTorrent(ctx context.Context, id string, destination string) (torrentName string, downloadDir string, err error) {
	switch destination {
	case "movies":
		downloadDir = m.moviesDir
	case "tvshows":
		downloadDir = m.tvshowsDir
	}

	if downloadDir == "" {
		return "", "", fmt.Errorf("wrong destination %s", destination)
	}

	torrent, err := m.FindTorrent(ctx, id)
	if err != nil {
		return "", "", err
	}

	torrentName = torrent.Name()
	magnetLink := torrent.MagnetLink()

	_, err = m.transmission.TorrentAdd(ctx, transmissionrpc.TorrentAddPayload{
		Filename:    &magnetLink,
		DownloadDir: &downloadDir,
	})
	if err != nil {
		return "", "", fmt.Errorf("add torrent %q: %w", torrentName, err)
	}

	return torrentName, downloadDir, nil
}

func (m *Media) FindTorrent(ctx context.Context, id string) (interfaces.TorrentSearchResult, error) {
	torrent, ok := m.cache.Get(id)

	if ok {
		return torrent, nil
	}

	torrent, err := m.searchClient.Find(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("find torrent %q: %w", id, err)
	}

	m.cache.Set(id, torrent)

	return torrent, nil
}

func (m *Media) SearchTorrent(ctx context.Context, query string) ([]interfaces.TorrentSearchResult, error) {
	torrentSearch, err := m.searchClient.Search(ctx, query, m.maxSearchResults)

	if err != nil {
		return nil, fmt.Errorf("search torrents for %q: %w", query, err)
	}

	return torrentSearch, nil
}

func (m *Media) SiteUrl(t interfaces.TorrentSearchResult) string {
	return m.searchClient.SiteUrl(t)
}

func (m *Media) FreeSpace(ctx context.Context) (cunits.Bits, error) {
	freeSpace, _, err := m.transmission.FreeSpace(ctx, m.rootDir)
	if err != nil {
		return freeSpace, fmt.Errorf("get free space for %q: %w", m.rootDir, err)
	}

	return freeSpace, nil
}
