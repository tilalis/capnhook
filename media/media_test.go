package media

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/hekmon/cunits/v2"
	"github.com/hekmon/transmissionrpc/v3"
	"github.com/tilalis/capnhook/media/piratebay"
)

func ptr[T any](v T) *T { return &v }

// --- mocks ---

type mockPiratebay struct {
	findFunc   func(ctx context.Context, id string) (*piratebay.Torrent, error)
	searchFunc func(ctx context.Context, query string) ([]piratebay.Torrent, error)
	siteURL    func(t *piratebay.Torrent) string
	findCalls  int
}

func (m *mockPiratebay) Find(ctx context.Context, id string) (*piratebay.Torrent, error) {
	m.findCalls++
	return m.findFunc(ctx, id)
}

func (m *mockPiratebay) Search(ctx context.Context, query string) ([]piratebay.Torrent, error) {
	return m.searchFunc(ctx, query)
}

func (m *mockPiratebay) SiteUrl(t *piratebay.Torrent) string {
	if m.siteURL == nil {
		return ""
	}
	return m.siteURL(t)
}

type mockTransmission struct {
	getByIDFunc   func(ctx context.Context, id int64) (transmissionrpc.Torrent, error)
	getAllFunc    func(ctx context.Context) ([]transmissionrpc.Torrent, error)
	addFunc       func(ctx context.Context, p transmissionrpc.TorrentAddPayload) (transmissionrpc.Torrent, error)
	removeFunc    func(ctx context.Context, p transmissionrpc.TorrentRemovePayload) error
	freeSpaceFunc func(ctx context.Context, path string) (cunits.Bits, cunits.Bits, error)

	addPayload    transmissionrpc.TorrentAddPayload
	removePayload transmissionrpc.TorrentRemovePayload
}

func (m *mockTransmission) TorrentGetByID(ctx context.Context, id int64) (transmissionrpc.Torrent, error) {
	return m.getByIDFunc(ctx, id)
}

func (m *mockTransmission) TorrentGetAll(ctx context.Context) ([]transmissionrpc.Torrent, error) {
	return m.getAllFunc(ctx)
}

func (m *mockTransmission) TorrentAdd(ctx context.Context, p transmissionrpc.TorrentAddPayload) (transmissionrpc.Torrent, error) {
	m.addPayload = p
	return m.addFunc(ctx, p)
}

func (m *mockTransmission) TorrentRemove(ctx context.Context, p transmissionrpc.TorrentRemovePayload) error {
	m.removePayload = p
	return m.removeFunc(ctx, p)
}

func (m *mockTransmission) FreeSpace(ctx context.Context, path string) (cunits.Bits, cunits.Bits, error) {
	return m.freeSpaceFunc(ctx, path)
}

// --- tests ---

func TestFindTorrent(t *testing.T) {
	t.Run("maps fields and caches result", func(t *testing.T) {
		pb := &mockPiratebay{
			findFunc: func(ctx context.Context, id string) (*piratebay.Torrent, error) {
				return &piratebay.Torrent{
					ID:       "42",
					Name:     "Ubuntu",
					InfoHash: "HASH",
					Size:     "1073741824",
					NumFiles: "3",
					Username: "seeder",
					Added:    "1700000000",
					Descr:    "desc",
				}, nil
			},
			siteURL: func(t *piratebay.Torrent) string {
				return "https://site.example/description.php?id=" + t.ID.String()
			},
		}
		m := New("/root", pb, nil, 0)

		got, err := m.FindTorrent(context.Background(), "42")
		if err != nil {
			t.Fatalf("FindTorrent() error = %v", err)
		}

		if got.ID != "42" || got.Name != "Ubuntu" || got.NumFiles != 3 ||
			got.Username != "seeder" || got.Description != "desc" {
			t.Errorf("unexpected mapping: %+v", got)
		}
		if got.SizeGB != 1.0 {
			t.Errorf("SizeGB = %v, want 1", got.SizeGB)
		}
		if !strings.HasPrefix(got.MagnetLink, "magnet:?xt=urn:btih:HASH") {
			t.Errorf("MagnetLink = %q, want btih:HASH prefix", got.MagnetLink)
		}
		if got.SiteUrl != "https://site.example/description.php?id=42" {
			t.Errorf("SiteUrl = %q", got.SiteUrl)
		}

		// A second lookup must be served from the cache, not the client.
		if _, err := m.FindTorrent(context.Background(), "42"); err != nil {
			t.Fatalf("FindTorrent() second call error = %v", err)
		}
		if pb.findCalls != 1 {
			t.Errorf("piratebay.Find called %d times, want 1 (second call should be cached)", pb.findCalls)
		}
	})

	t.Run("propagates lookup error", func(t *testing.T) {
		pb := &mockPiratebay{
			findFunc: func(ctx context.Context, id string) (*piratebay.Torrent, error) {
				return nil, errors.New("boom")
			},
		}
		m := New("/root", pb, nil, 0)
		if _, err := m.FindTorrent(context.Background(), "42"); err == nil {
			t.Fatal("FindTorrent() expected error, got nil")
		}
	})
}

func TestSearchTorrent(t *testing.T) {
	t.Run("maps fields and truncates to max results", func(t *testing.T) {
		pb := &mockPiratebay{
			searchFunc: func(ctx context.Context, query string) ([]piratebay.Torrent, error) {
				return []piratebay.Torrent{
					{ID: "1", Name: "A", Size: "1073741824", NumFiles: "1", Username: "u", Added: "1"},
					{ID: "2", Name: "B", Size: "0", NumFiles: "2", Username: "v", Added: "2"},
					{ID: "3", Name: "C", Size: "0", NumFiles: "3", Username: "w", Added: "3"},
				}, nil
			},
		}
		m := New("/root", pb, nil, 2)

		got, err := m.SearchTorrent(context.Background(), "q")
		if err != nil {
			t.Fatalf("SearchTorrent() error = %v", err)
		}

		if len(got) != 2 {
			t.Fatalf("len(got) = %d, want 2 (capped at maxSearchResults)", len(got))
		}
		if got[0].Name != "A" || got[0].NumFiles != 1 || got[0].SizeGB != 1.0 {
			t.Errorf("unexpected first result: %+v", got[0])
		}
	})

	t.Run("propagates search error", func(t *testing.T) {
		pb := &mockPiratebay{
			searchFunc: func(ctx context.Context, query string) ([]piratebay.Torrent, error) {
				return nil, errors.New("boom")
			},
		}
		m := New("/root", pb, nil, 0)
		if _, err := m.SearchTorrent(context.Background(), "q"); err == nil {
			t.Fatal("SearchTorrent() expected error, got nil")
		}
	})
}

func TestDownloadTorrent(t *testing.T) {
	newMedia := func(add func(ctx context.Context, p transmissionrpc.TorrentAddPayload) (transmissionrpc.Torrent, error)) (*Media, *mockTransmission) {
		pb := &mockPiratebay{
			findFunc: func(ctx context.Context, id string) (*piratebay.Torrent, error) {
				return &piratebay.Torrent{ID: "42", Name: "Ubuntu", InfoHash: "HASH", NumFiles: "1"}, nil
			},
		}
		tr := &mockTransmission{addFunc: add}
		return New("/root", pb, tr, 0), tr
	}

	okAdd := func(ctx context.Context, p transmissionrpc.TorrentAddPayload) (transmissionrpc.Torrent, error) {
		return transmissionrpc.Torrent{}, nil
	}

	t.Run("routes movies to Movies dir and adds magnet", func(t *testing.T) {
		m, tr := newMedia(okAdd)

		name, dir, err := m.DownloadTorrent(context.Background(), "42", "movies")
		if err != nil {
			t.Fatalf("DownloadTorrent() error = %v", err)
		}
		if name != "Ubuntu" {
			t.Errorf("name = %q, want Ubuntu", name)
		}
		if dir != "/root/Movies" {
			t.Errorf("dir = %q, want /root/Movies", dir)
		}
		if tr.addPayload.DownloadDir == nil || *tr.addPayload.DownloadDir != "/root/Movies" {
			t.Errorf("TorrentAdd DownloadDir = %v, want /root/Movies", tr.addPayload.DownloadDir)
		}
		if tr.addPayload.Filename == nil || !strings.HasPrefix(*tr.addPayload.Filename, "magnet:?xt=urn:btih:HASH") {
			t.Errorf("TorrentAdd Filename = %v, want magnet link", tr.addPayload.Filename)
		}
	})

	t.Run("routes tvshows to TVShows dir", func(t *testing.T) {
		m, _ := newMedia(okAdd)

		_, dir, err := m.DownloadTorrent(context.Background(), "42", "tvshows")
		if err != nil {
			t.Fatalf("DownloadTorrent() error = %v", err)
		}
		if dir != "/root/TVShows" {
			t.Errorf("dir = %q, want /root/TVShows", dir)
		}
	})

	t.Run("wrong destination returns error without adding", func(t *testing.T) {
		m, _ := newMedia(func(ctx context.Context, p transmissionrpc.TorrentAddPayload) (transmissionrpc.Torrent, error) {
			t.Error("TorrentAdd should not be called for wrong destination")
			return transmissionrpc.Torrent{}, nil
		})

		_, _, err := m.DownloadTorrent(context.Background(), "42", "invalid")
		if err == nil {
			t.Fatal("DownloadTorrent() expected error for invalid destination, got nil")
		}
		if !strings.Contains(err.Error(), "wrong destination") {
			t.Errorf("error = %v, want to mention 'wrong destination'", err)
		}
	})

	t.Run("propagates add error", func(t *testing.T) {
		m, _ := newMedia(func(ctx context.Context, p transmissionrpc.TorrentAddPayload) (transmissionrpc.Torrent, error) {
			return transmissionrpc.Torrent{}, errors.New("boom")
		})
		if _, _, err := m.DownloadTorrent(context.Background(), "42", "movies"); err == nil {
			t.Fatal("DownloadTorrent() expected error from TorrentAdd, got nil")
		}
	})
}

func TestDeleteCurrentTorrent(t *testing.T) {
	t.Run("removes with local data and returns name", func(t *testing.T) {
		tr := &mockTransmission{
			getByIDFunc: func(ctx context.Context, id int64) (transmissionrpc.Torrent, error) {
				return transmissionrpc.Torrent{ID: ptr(int64(7)), Name: ptr("Ubuntu")}, nil
			},
			removeFunc: func(ctx context.Context, p transmissionrpc.TorrentRemovePayload) error {
				return nil
			},
		}
		m := New("/root", nil, tr, 0)

		name, err := m.DeleteCurrentTorrent(context.Background(), "7")
		if err != nil {
			t.Fatalf("DeleteCurrentTorrent() error = %v", err)
		}
		if name != "Ubuntu" {
			t.Errorf("name = %q, want Ubuntu", name)
		}
		if len(tr.removePayload.IDs) != 1 || tr.removePayload.IDs[0] != 7 {
			t.Errorf("remove IDs = %v, want [7]", tr.removePayload.IDs)
		}
		if !tr.removePayload.DeleteLocalData {
			t.Error("DeleteLocalData = false, want true")
		}
	})

	t.Run("invalid id returns error", func(t *testing.T) {
		m := New("/root", nil, &mockTransmission{}, 0)
		if _, err := m.DeleteCurrentTorrent(context.Background(), "abc"); err == nil {
			t.Fatal("DeleteCurrentTorrent() expected error for invalid id, got nil")
		}
	})

	t.Run("missing fields yields bad response error", func(t *testing.T) {
		tr := &mockTransmission{
			getByIDFunc: func(ctx context.Context, id int64) (transmissionrpc.Torrent, error) {
				return transmissionrpc.Torrent{ID: ptr(int64(7))}, nil // Name nil
			},
		}
		m := New("/root", nil, tr, 0)
		if _, err := m.DeleteCurrentTorrent(context.Background(), "7"); !errors.Is(err, errBadTransmissionRPCResponse) {
			t.Fatalf("error = %v, want errBadTransmissionRPCResponse", err)
		}
	})
}

func TestGetCurrentTorrent(t *testing.T) {
	t.Run("maps status fields", func(t *testing.T) {
		tr := &mockTransmission{
			getByIDFunc: func(ctx context.Context, id int64) (transmissionrpc.Torrent, error) {
				return transmissionrpc.Torrent{
					ID:           ptr(int64(7)),
					Name:         ptr("Ubuntu"),
					PercentDone:  ptr(0.5),
					ETA:          ptr(int64(3600)),
					SizeWhenDone: ptr(cunits.Bits(1073741824)),
				}, nil
			},
		}
		m := New("/root", nil, tr, 0)

		st, err := m.GetCurrentTorrent(context.Background(), "7")
		if err != nil {
			t.Fatalf("GetCurrentTorrent() error = %v", err)
		}
		if st.ID != 7 || st.Name != "Ubuntu" {
			t.Errorf("unexpected status: %+v", st)
		}
		if st.PercentDone != 50 {
			t.Errorf("PercentDone = %v, want 50", st.PercentDone)
		}
		if st.ETA != time.Hour {
			t.Errorf("ETA = %v, want 1h", st.ETA)
		}
		if st.Done {
			t.Error("Done = true, want false at 50%")
		}
	})

	t.Run("invalid id returns error", func(t *testing.T) {
		m := New("/root", nil, &mockTransmission{}, 0)
		if _, err := m.GetCurrentTorrent(context.Background(), "abc"); err == nil {
			t.Fatal("GetCurrentTorrent() expected error for invalid id, got nil")
		}
	})

	t.Run("missing field yields bad response error", func(t *testing.T) {
		tr := &mockTransmission{
			getByIDFunc: func(ctx context.Context, id int64) (transmissionrpc.Torrent, error) {
				return transmissionrpc.Torrent{
					ID: ptr(int64(7)), Name: ptr("x"), PercentDone: ptr(1.0), SizeWhenDone: ptr(cunits.Bits(1)),
				}, nil // ETA nil
			},
		}
		m := New("/root", nil, tr, 0)
		if _, err := m.GetCurrentTorrent(context.Background(), "7"); !errors.Is(err, errBadTransmissionRPCResponse) {
			t.Fatalf("error = %v, want errBadTransmissionRPCResponse", err)
		}
	})
}

func TestGetCurrentTorrents(t *testing.T) {
	torrents := func() []transmissionrpc.Torrent {
		return []transmissionrpc.Torrent{
			{ID: ptr(int64(1)), Name: ptr("done"), PercentDone: ptr(1.0), ETA: ptr(int64(0)), SizeWhenDone: ptr(cunits.Bits(100))},
			{ID: ptr(int64(2)), Name: ptr("wip"), PercentDone: ptr(0.5), ETA: ptr(int64(3600)), SizeWhenDone: ptr(cunits.Bits(200))},
		}
	}

	t.Run("includes all with Done flag when not filtering", func(t *testing.T) {
		tr := &mockTransmission{getAllFunc: func(ctx context.Context) ([]transmissionrpc.Torrent, error) {
			return torrents(), nil
		}}
		m := New("/root", nil, tr, 0)

		got, err := m.GetCurrentTorrents(context.Background(), false)
		if err != nil {
			t.Fatalf("GetCurrentTorrents() error = %v", err)
		}
		if len(got) != 2 {
			t.Fatalf("len(got) = %d, want 2", len(got))
		}
		if !got[0].Done {
			t.Error("first torrent Done = false, want true (100%)")
		}
		if got[1].Done {
			t.Error("second torrent Done = true, want false (50%)")
		}
	})

	t.Run("skips completed torrents when inProgress", func(t *testing.T) {
		tr := &mockTransmission{getAllFunc: func(ctx context.Context) ([]transmissionrpc.Torrent, error) {
			return torrents(), nil
		}}
		m := New("/root", nil, tr, 0)

		got, err := m.GetCurrentTorrents(context.Background(), true)
		if err != nil {
			t.Fatalf("GetCurrentTorrents() error = %v", err)
		}
		if len(got) != 1 {
			t.Fatalf("len(got) = %d, want 1 (completed skipped)", len(got))
		}
		if got[0].Name != "wip" {
			t.Errorf("got %q, want 'wip'", got[0].Name)
		}
	})

	t.Run("missing field yields bad response error", func(t *testing.T) {
		tr := &mockTransmission{getAllFunc: func(ctx context.Context) ([]transmissionrpc.Torrent, error) {
			return []transmissionrpc.Torrent{{ID: ptr(int64(1))}}, nil
		}}
		m := New("/root", nil, tr, 0)
		if _, err := m.GetCurrentTorrents(context.Background(), false); !errors.Is(err, errBadTransmissionRPCResponse) {
			t.Fatalf("error = %v, want errBadTransmissionRPCResponse", err)
		}
	})
}

func TestFreeSpace(t *testing.T) {
	t.Run("returns free space for the root dir", func(t *testing.T) {
		var gotPath string
		tr := &mockTransmission{freeSpaceFunc: func(ctx context.Context, path string) (cunits.Bits, cunits.Bits, error) {
			gotPath = path
			return cunits.Bits(4096), cunits.Bits(8192), nil
		}}
		m := New("/root", nil, tr, 0)

		fs, err := m.FreeSpace(context.Background())
		if err != nil {
			t.Fatalf("FreeSpace() error = %v", err)
		}
		if fs != cunits.Bits(4096) {
			t.Errorf("free space = %v, want 4096", fs)
		}
		if gotPath != "/root" {
			t.Errorf("queried path = %q, want /root", gotPath)
		}
	})

	t.Run("propagates error", func(t *testing.T) {
		tr := &mockTransmission{freeSpaceFunc: func(ctx context.Context, path string) (cunits.Bits, cunits.Bits, error) {
			return 0, 0, errors.New("boom")
		}}
		m := New("/root", nil, tr, 0)
		if _, err := m.FreeSpace(context.Background()); err == nil {
			t.Fatal("FreeSpace() expected error, got nil")
		}
	})
}
