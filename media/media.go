package media

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/hekmon/cunits/v2"
	"github.com/hekmon/transmissionrpc/v3"
	"github.com/scalalang2/golang-fifo/sieve"
	"github.com/tilalis/capnhook/media/clients/apibay"
	"github.com/tilalis/capnhook/media/clients/internetarchive"
	"github.com/tilalis/capnhook/media/interfaces"
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
	TorrentRenamePath(ctx context.Context, id int64, path, name string) error
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

// NewDefault builds a Media backed by the search client selected by
// searchClientName (see NewSearchClient) and a Transmission client at
// transmissionURL. rootDir and transmissionURL are required; a non-positive
// maxSearchResults falls back to the built-in default.
func NewDefault(rootDir, transmissionURL, searchClientName string, maxSearchResults int) (*Media, error) {
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

	searchClient, err := NewSearchClient(searchClientName)
	if err != nil {
		return nil, err
	}

	return New(rootDir, searchClient, tr, maxSearchResults), nil
}

// NewSearchClient builds a search client by name. An empty name defaults to
// "apibay"; "internetarchive" searches permissibly licensed movies on
// archive.org.
func NewSearchClient(name string) (interfaces.SearchClient, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "", "apibay":
		return apibay.NewDefault(), nil
	case "internetarchive":
		return internetarchive.NewDefault(), nil
	default:
		return nil, fmt.Errorf("unknown search client %q", name)
	}
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

	payload := transmissionrpc.TorrentAddPayload{DownloadDir: &downloadDir}

	// Prefer handing the raw torrent contents to Transmission: the daemon may
	// not be able to reach the search backend's servers itself.
	if provider, ok := torrent.(interfaces.TorrentFileProvider); ok {
		contents, err := provider.TorrentFile(ctx)
		if err != nil {
			return "", "", fmt.Errorf("fetch torrent file for %q: %w", torrentName, err)
		}

		metainfo := base64.StdEncoding.EncodeToString(contents)
		payload.MetaInfo = &metainfo
	} else {
		magnetLink := torrent.MagnetLink()
		payload.Filename = &magnetLink
	}

	added, err := m.transmission.TorrentAdd(ctx, payload)
	if err != nil {
		return "", "", fmt.Errorf("add torrent %q: %w", torrentName, err)
	}

	// Torrents added by contents keep the name embedded in the file — for
	// Internet Archive items that is the item identifier, often an opaque or
	// numeric string. Rename to the human-readable title so both Transmission
	// and the media library show it.
	if payload.MetaInfo != nil {
		m.renameTorrent(ctx, added, torrentName)
	}

	return torrentName, downloadDir, nil
}

// renameTorrent renames the added torrent's root path to the search result's
// title. Best effort: the download is already running, so failures are only
// logged.
func (m *Media) renameTorrent(ctx context.Context, torrent transmissionrpc.Torrent, title string) {
	name := sanitizeTorrentName(title)
	if name == "" || torrent.ID == nil || torrent.Name == nil || *torrent.Name == name {
		return
	}

	if err := m.transmission.TorrentRenamePath(ctx, *torrent.ID, *torrent.Name, name); err != nil {
		slog.WarnContext(ctx, "failed to rename torrent", "from", *torrent.Name, "to", name, "error", err)
	}
}

// sanitizeTorrentName makes a search-result title safe to use as a file or
// directory name on the download host.
func sanitizeTorrentName(title string) string {
	sanitized := strings.Map(func(r rune) rune {
		if r < 32 || strings.ContainsRune(`/\:*?"<>|`, r) {
			return ' '
		}
		return r
	}, title)

	sanitized = strings.Join(strings.Fields(sanitized), " ")

	return strings.Trim(sanitized, " .")
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

func (m *Media) SearchTorrents(ctx context.Context, query string) ([]interfaces.TorrentSearchResult, error) {
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
