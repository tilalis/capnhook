package media

import (
	"context"
	"encoding/base64"
	"testing"
	"time"

	"github.com/hekmon/cunits/v2"
	"github.com/hekmon/transmissionrpc/v3"
	"github.com/tilalis/capnhook/media/interfaces"
)

// fakeResult implements interfaces.TorrentSearchResult; with torrentFile set
// it also implements interfaces.TorrentFileProvider.
type fakeResult struct {
	name        string
	magnet      string
	torrentFile []byte
}

func (f fakeResult) ID() string           { return "fake" }
func (f fakeResult) Name() string         { return f.name }
func (f fakeResult) SizeGB() float64      { return 1 }
func (f fakeResult) NumFiles() int64      { return 1 }
func (f fakeResult) AddedTime() time.Time { return time.Time{} }
func (f fakeResult) Username() string     { return "user" }
func (f fakeResult) Description() string  { return "" }
func (f fakeResult) MagnetLink() string   { return f.magnet }

type fileResult struct {
	fakeResult
}

func (f fileResult) TorrentFile(ctx context.Context) ([]byte, error) {
	return f.torrentFile, nil
}

type fakeSearchClient struct {
	result interfaces.TorrentSearchResult
}

func (f fakeSearchClient) Find(ctx context.Context, id string) (interfaces.TorrentSearchResult, error) {
	return f.result, nil
}

func (f fakeSearchClient) Search(ctx context.Context, query string, limit int) ([]interfaces.TorrentSearchResult, error) {
	return []interfaces.TorrentSearchResult{f.result}, nil
}

func (f fakeSearchClient) SiteUrl(t interfaces.TorrentSearchResult) string { return "" }

type renameCall struct {
	id         int64
	path, name string
}

type fakeTransmission struct {
	addedName    string
	addedPayload transmissionrpc.TorrentAddPayload
	renames      []renameCall
}

func (f *fakeTransmission) TorrentAdd(ctx context.Context, payload transmissionrpc.TorrentAddPayload) (transmissionrpc.Torrent, error) {
	f.addedPayload = payload
	id := int64(7)
	return transmissionrpc.Torrent{ID: &id, Name: &f.addedName}, nil
}

func (f *fakeTransmission) TorrentRenamePath(ctx context.Context, id int64, path, name string) error {
	f.renames = append(f.renames, renameCall{id, path, name})
	return nil
}

func (f *fakeTransmission) TorrentGetByID(ctx context.Context, id int64) (transmissionrpc.Torrent, error) {
	return transmissionrpc.Torrent{}, nil
}

func (f *fakeTransmission) TorrentGetAll(ctx context.Context) ([]transmissionrpc.Torrent, error) {
	return nil, nil
}

func (f *fakeTransmission) TorrentRemove(ctx context.Context, payload transmissionrpc.TorrentRemovePayload) error {
	return nil
}

func (f *fakeTransmission) FreeSpace(ctx context.Context, path string) (cunits.Bits, cunits.Bits, error) {
	return 0, 0, nil
}

func TestDownloadTorrentByContentsRenamesToTitle(t *testing.T) {
	tr := &fakeTransmission{addedName: "0612391"}
	result := fileResult{fakeResult{
		name:        ` A Bucket: of/Blood (1959) `,
		torrentFile: []byte("raw torrent contents"),
	}}

	m := New("/plex", fakeSearchClient{result: result}, tr, 0)

	torrentName, _, err := m.DownloadTorrent(context.Background(), "fake", "movies")
	if err != nil {
		t.Fatalf("DownloadTorrent: %v", err)
	}
	if torrentName != result.name {
		t.Errorf("torrentName = %q, want the search result title", torrentName)
	}

	if f := tr.addedPayload.Filename; f != nil {
		t.Errorf("Filename = %q, want torrent added by contents instead", *f)
	}
	if mi := tr.addedPayload.MetaInfo; mi == nil || *mi != base64.StdEncoding.EncodeToString(result.torrentFile) {
		t.Errorf("MetaInfo is not the base64 of the torrent file contents")
	}

	want := renameCall{id: 7, path: "0612391", name: "A Bucket of Blood (1959)"}
	if len(tr.renames) != 1 || tr.renames[0] != want {
		t.Errorf("renames = %+v, want exactly %+v", tr.renames, want)
	}
}

func TestDownloadTorrentByMagnetDoesNotRename(t *testing.T) {
	tr := &fakeTransmission{addedName: "Some Torrent"}
	result := fakeResult{name: "Some Torrent", magnet: "magnet:?xt=urn:btih:HASH"}

	m := New("/plex", fakeSearchClient{result: result}, tr, 0)

	if _, _, err := m.DownloadTorrent(context.Background(), "fake", "movies"); err != nil {
		t.Fatalf("DownloadTorrent: %v", err)
	}

	if f := tr.addedPayload.Filename; f == nil || *f != result.magnet {
		t.Errorf("Filename = %v, want the magnet link", f)
	}
	if len(tr.renames) != 0 {
		t.Errorf("renames = %+v, want none for magnet adds", tr.renames)
	}
}
