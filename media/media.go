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
	"github.com/tilalis/capnhook/media/piratebay"
	"github.com/tilalis/capnhook/media/transmission"
)

type Media struct {
	piratebay                      *piratebay.Piratebay
	transmission                   *transmission.Transmission
	cache                          *sieve.Sieve[string, *TorrentSearch]
	rootDir, moviesDir, tvshowsDir string
}

type TorrentSearch struct {
	ID          string
	Name        string
	SizeGB      float64
	NumFiles    int
	Added       time.Time
	Username    string
	Description string
	MagnetLink  string
	SiteUrl     string
}

type TorrentStatus struct {
	ID           int64
	Name         string
	PercentDone  float64
	ETA          time.Duration
	SizeWhenDone cunits.Bits
	Done         bool
}

func New(root string, p *piratebay.Piratebay, tr *transmission.Transmission) *Media {
	return &Media{
		piratebay:    p,
		transmission: tr,
		cache:        sieve.New[string, *TorrentSearch](16, 0),
		rootDir:      root,
		moviesDir:    path.Join(root, "Movies"),
		tvshowsDir:   path.Join(root, "TVShows"),
	}
}

func NewDefault() (*Media, error) {
	tr, err := transmission.NewTransmissionClient("")
	if err != nil {
		return nil, err
	}
	return New("/home/tilalis/Plex", piratebay.NewDefault(), tr), nil
}

var errBadTransmissionRPCResponse = errors.New("bad Transmission RPC response")

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

func (m *Media) GetCurrentTorrent(ctx context.Context, id string) (*TorrentStatus, error) {
	identifier, err := strconv.Atoi(id)
	if err != nil {
		return nil, fmt.Errorf("invalid torrent id %q: %w", id, err)
	}

	torrent, err := m.transmission.TorrentGetByID(ctx, int64(identifier))
	if err != nil {
		return nil, fmt.Errorf("get torrent %d: %w", identifier, err)
	}

	if torrent.ID == nil || torrent.Name == nil || torrent.SizeWhenDone == nil || torrent.PercentDone == nil {
		return nil, errBadTransmissionRPCResponse
	}

	return &TorrentStatus{
		ID:           *torrent.ID,
		Name:         *torrent.Name,
		PercentDone:  *torrent.PercentDone * 100,
		SizeWhenDone: *torrent.SizeWhenDone,
	}, nil
}

func (m *Media) GetCurrentTorrents(ctx context.Context, inProgress bool) ([]TorrentStatus, error) {
	torrents, err := m.transmission.TorrentGetAll(ctx)

	if err != nil {
		return nil, fmt.Errorf("get all torrents: %w", err)
	}

	var torrentStatuses []TorrentStatus = make([]TorrentStatus, 0, len(torrents))

	for _, torrent := range torrents {
		if torrent.ID == nil || torrent.Name == nil || torrent.SizeWhenDone == nil || torrent.PercentDone == nil || torrent.ETA == nil {
			return nil, errBadTransmissionRPCResponse
		}

		done := false
		percentDone := *torrent.PercentDone

		if percentDone == 1.0 {
			if inProgress {
				continue
			}
			done = true
		}

		var eta time.Duration
		rawEta := *torrent.ETA

		if rawEta > 0 {
			eta = time.Duration(rawEta) * time.Second
		}
		torrentStatuses = append(torrentStatuses, TorrentStatus{
			ID:           *torrent.ID,
			Name:         *torrent.Name,
			PercentDone:  *torrent.PercentDone * 100,
			SizeWhenDone: *torrent.SizeWhenDone,
			ETA:          eta,
			Done:         done,
		})
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

	torrentName = torrent.Name

	_, err = m.transmission.TorrentAdd(ctx, transmissionrpc.TorrentAddPayload{
		Filename:    &torrent.MagnetLink,
		DownloadDir: &downloadDir,
	})
	if err != nil {
		return "", "", fmt.Errorf("add torrent %q: %w", torrentName, err)
	}

	return torrentName, downloadDir, nil
}

func (m *Media) FindTorrent(ctx context.Context, id string) (*TorrentSearch, error) {
	torrent, ok := m.cache.Get(id)

	if ok {
		return torrent, nil
	}

	piratebayTorrent, err := m.piratebay.Find(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("find torrent %q: %w", id, err)
	}

	numFiles, err := piratebayTorrent.NumFiles.Int64()
	if err != nil {
		return nil, fmt.Errorf("parse num_files for torrent %q: %w", id, err)
	}

	torrent = &TorrentSearch{
		ID:          piratebayTorrent.ID.String(),
		Name:        piratebayTorrent.Name,
		SizeGB:      piratebayTorrent.SizeGB(),
		NumFiles:    int(numFiles),
		Added:       piratebayTorrent.AddedTime(),
		Username:    piratebayTorrent.Username,
		Description: piratebayTorrent.Descr,
		MagnetLink:  piratebayTorrent.MagnetLink(),
		SiteUrl:     m.piratebay.SiteUrl(piratebayTorrent),
	}

	m.cache.Set(id, torrent)

	return torrent, nil
}

func (m *Media) SearchTorrent(ctx context.Context, query string) ([]TorrentSearch, error) {
	torrents, err := m.piratebay.Search(ctx, query)

	if err != nil {
		return nil, fmt.Errorf("search torrents for %q: %w", query, err)
	}

	if len(torrents) >= 30 {
		torrents = torrents[:30]
	}

	var torrentSearch []TorrentSearch = make([]TorrentSearch, 0, 30)

	for _, torrent := range torrents {
		numFiles, err := torrent.NumFiles.Int64()

		if err != nil {
			return nil, fmt.Errorf("parse num_files for torrent %q: %w", torrent.ID, err)
		}

		torrentSearch = append(torrentSearch, TorrentSearch{
			ID:          torrent.ID.String(),
			Name:        torrent.Name,
			SizeGB:      torrent.SizeGB(),
			NumFiles:    int(numFiles),
			Added:       torrent.AddedTime(),
			Username:    torrent.Username,
			Description: torrent.Descr,
		})
	}

	return torrentSearch, nil
}

func (m *Media) FreeSpace(ctx context.Context) (cunits.Bits, error) {
	freeSpace, _, err := m.transmission.FreeSpace(ctx, m.rootDir)
	if err != nil {
		return freeSpace, fmt.Errorf("get free space for %q: %w", m.rootDir, err)
	}

	return freeSpace, nil
}
