package piratebay

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFind(t *testing.T) {
	t.Run("success shapes request and parses response", func(t *testing.T) {
		var gotPath, gotID, gotUA string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			gotID = r.URL.Query().Get("id")
			gotUA = r.Header.Get("User-Agent")
			w.Write([]byte(`{"id":"42","name":"Ubuntu","info_hash":"HASH","size":"1073741824","num_files":"3","username":"seeder","added":"1700000000","descr":"desc"}`))
		}))
		defer srv.Close()

		p := New(srv.URL, "")
		torrent, err := p.Find(context.Background(), "42")
		if err != nil {
			t.Fatalf("Find() error = %v", err)
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
		if torrent.Name != "Ubuntu" || torrent.InfoHash != "HASH" {
			t.Errorf("unexpected torrent: %+v", torrent)
		}
	})

	t.Run("not found sentinel returns error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`{"id":"0","name":"","info_hash":"0000000000000000000000000000000000000000","size":"0","num_files":"0","username":"","added":"0"}`))
		}))
		defer srv.Close()

		p := New(srv.URL, "")
		_, err := p.Find(context.Background(), "999")
		if err == nil {
			t.Fatal("Find() expected error for empty torrent, got nil")
		}
		if !strings.Contains(err.Error(), "not found") {
			t.Errorf("error = %v, want to mention 'not found'", err)
		}
	})

	t.Run("http error status returns error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer srv.Close()

		p := New(srv.URL, "")
		if _, err := p.Find(context.Background(), "42"); err == nil {
			t.Fatal("Find() expected error for HTTP 500, got nil")
		}
	})

	t.Run("invalid json returns error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`not json`))
		}))
		defer srv.Close()

		p := New(srv.URL, "")
		if _, err := p.Find(context.Background(), "42"); err == nil {
			t.Fatal("Find() expected error for invalid JSON, got nil")
		}
	})
}

func TestSearch(t *testing.T) {
	t.Run("success escapes query and parses list", func(t *testing.T) {
		var gotPath, gotQuery string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			gotQuery = r.URL.Query().Get("q")
			w.Write([]byte(`[{"id":"1","name":"A","size":"100","num_files":"1","username":"u","added":"1"},{"id":"2","name":"B","size":"200","num_files":"2","username":"v","added":"2"}]`))
		}))
		defer srv.Close()

		p := New(srv.URL, "")
		torrents, err := p.Search(context.Background(), "ubuntu iso")
		if err != nil {
			t.Fatalf("Search() error = %v", err)
		}

		if gotPath != "/q.php" {
			t.Errorf("request path = %q, want /q.php", gotPath)
		}
		if gotQuery != "ubuntu iso" {
			t.Errorf("request q = %q, want 'ubuntu iso'", gotQuery)
		}
		if len(torrents) != 2 {
			t.Fatalf("len(torrents) = %d, want 2", len(torrents))
		}
		if torrents[0].Name != "A" || torrents[1].Name != "B" {
			t.Errorf("unexpected torrents: %+v", torrents)
		}
	})

	t.Run("empty result sentinel returns error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`[{"id":"0","name":"No results returned","info_hash":"0000000000000000000000000000000000000000","size":"0","num_files":"0","username":"","added":"0"}]`))
		}))
		defer srv.Close()

		p := New(srv.URL, "")
		_, err := p.Search(context.Background(), "nonexistent")
		if err == nil {
			t.Fatal("Search() expected error for empty result, got nil")
		}
		if !strings.Contains(err.Error(), "no torrents found") {
			t.Errorf("error = %v, want to mention 'no torrents found'", err)
		}
	})

	t.Run("http error status returns error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadGateway)
		}))
		defer srv.Close()

		p := New(srv.URL, "")
		if _, err := p.Search(context.Background(), "x"); err == nil {
			t.Fatal("Search() expected error for HTTP 502, got nil")
		}
	})

	t.Run("invalid json returns error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`{bad`))
		}))
		defer srv.Close()

		p := New(srv.URL, "")
		if _, err := p.Search(context.Background(), "x"); err == nil {
			t.Fatal("Search() expected error for invalid JSON, got nil")
		}
	})
}

func TestFindCancelledContext(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()

	p := New(srv.URL, "")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := p.Find(ctx, "42"); err == nil {
		t.Fatal("Find() with cancelled context expected error, got nil")
	}
}
