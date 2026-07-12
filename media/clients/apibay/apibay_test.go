package apibay

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFind(t *testing.T) {
	var gotPath, gotID, gotUA string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotID = r.URL.Query().Get("id")
		gotUA = r.Header.Get("User-Agent")

		w.Write([]byte(`{
			"id": "42",
			"name": "Ubuntu ISO",
			"info_hash": "HASH",
			"size": "1073741824",
			"num_files": "3",
			"username": "seeder",
			"added": "1700000000",
			"descr": "desc"
		}`))
	}))
	defer server.Close()

	client := New(server.URL, "https://site.example")

	torrent, err := client.Find(context.Background(), "42")
	if err != nil {
		t.Fatalf("Find: %v", err)
	}

	if gotPath != "/t.php" {
		t.Errorf("request path = %q, want /t.php", gotPath)
	}
	if gotID != "42" {
		t.Errorf("request id = %q, want 42", gotID)
	}
	if gotUA == "" {
		t.Error("User-Agent header not set on request")
	}

	if torrent.ID() != "42" {
		t.Errorf("ID = %q", torrent.ID())
	}
	if torrent.Name() != "Ubuntu ISO" {
		t.Errorf("Name = %q", torrent.Name())
	}
	if torrent.SizeGB() != 1.0 {
		t.Errorf("SizeGB = %v, want 1.0", torrent.SizeGB())
	}
	if torrent.NumFiles() != 3 {
		t.Errorf("NumFiles = %d, want 3", torrent.NumFiles())
	}
	if torrent.Username() != "seeder" {
		t.Errorf("Username = %q", torrent.Username())
	}
	if torrent.AddedTime().Unix() != 1700000000 {
		t.Errorf("AddedTime = %v, want unix 1700000000", torrent.AddedTime())
	}
	if torrent.Description() != "desc" {
		t.Errorf("Description = %q", torrent.Description())
	}

	magnet := torrent.MagnetLink()
	if !strings.HasPrefix(magnet, "magnet:?xt=urn:btih:HASH&dn=Ubuntu+ISO") {
		t.Errorf("MagnetLink = %q, want prefix with info hash and encoded name", magnet)
	}
	if !strings.Contains(magnet, "&tr=") {
		t.Errorf("MagnetLink = %q, want trackers appended", magnet)
	}

	if want := "https://site.example/description.php?id=42"; client.SiteUrl(torrent) != want {
		t.Errorf("SiteUrl = %q, want %q", client.SiteUrl(torrent), want)
	}
}

func TestFindNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"id":"0","name":"","info_hash":"0000000000000000000000000000000000000000","size":"0","num_files":"0","username":"","added":"0"}`))
	}))
	defer server.Close()

	_, err := New(server.URL, "").Find(context.Background(), "999")
	if err == nil {
		t.Fatal("expected an error for the not-found sentinel response")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %v, want to mention 'not found'", err)
	}
}

func TestSearch(t *testing.T) {
	var gotPath, gotQuery string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.Query().Get("q")

		w.Write([]byte(`[
			{"id":"1","name":"A","size":"100","num_files":"1","username":"u","added":"1"},
			{"id":"2","name":"B","size":"200","num_files":"2","username":"v","added":"2"},
			{"id":"3","name":"C","size":"300","num_files":"3","username":"w","added":"3"}
		]`))
	}))
	defer server.Close()

	torrents, err := New(server.URL, "").Search(context.Background(), "ubuntu iso", 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}

	if gotPath != "/q.php" {
		t.Errorf("request path = %q, want /q.php", gotPath)
	}
	if gotQuery != "ubuntu iso" {
		t.Errorf("request q = %q, want 'ubuntu iso'", gotQuery)
	}

	if len(torrents) != 3 {
		t.Fatalf("got %d results, want 3", len(torrents))
	}
	if torrents[0].Name() != "A" || torrents[1].Name() != "B" || torrents[2].Name() != "C" {
		t.Errorf("unexpected result order: %q, %q, %q", torrents[0].Name(), torrents[1].Name(), torrents[2].Name())
	}
}

func TestSearchAppliesLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`[
			{"id":"1","name":"A","size":"100","num_files":"1","username":"u","added":"1"},
			{"id":"2","name":"B","size":"200","num_files":"2","username":"v","added":"2"},
			{"id":"3","name":"C","size":"300","num_files":"3","username":"w","added":"3"}
		]`))
	}))
	defer server.Close()

	torrents, err := New(server.URL, "").Search(context.Background(), "x", 2)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}

	if len(torrents) != 2 {
		t.Fatalf("got %d results, want 2 (limit applied)", len(torrents))
	}
	if torrents[0].Name() != "A" || torrents[1].Name() != "B" {
		t.Errorf("limit kept wrong results: %q, %q", torrents[0].Name(), torrents[1].Name())
	}
}

func TestSearchNoResults(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`[{"id":"0","name":"No results returned","info_hash":"0000000000000000000000000000000000000000","size":"0","num_files":"0","username":"","added":"0"}]`))
	}))
	defer server.Close()

	_, err := New(server.URL, "").Search(context.Background(), "nonexistent", 10)
	if err == nil {
		t.Fatal("expected an error for the empty-result sentinel response")
	}
	if !strings.Contains(err.Error(), "no torrents found") {
		t.Errorf("error = %v, want to mention 'no torrents found'", err)
	}
}
