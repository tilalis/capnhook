package internetarchive

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSearch(t *testing.T) {
	var gotPath, gotQuery string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.Query().Get("q")

		w.Write([]byte(`{
			"response": {
				"docs": [
					{
						"identifier": "night_of_the_living_dead",
						"title": "Night of the Living Dead",
						"description": ["A classic.", "Public domain."],
						"creator": "George A. Romero",
						"item_size": 1610612736,
						"files_count": 12,
						"publicdate": "2004-03-19T00:00:00Z"
					}
				]
			}
		}`))
	}))
	defer server.Close()

	client := New(server.URL, server.URL)

	results, err := client.Search(context.Background(), `night "of: the`, 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}

	if gotPath != "/advancedsearch.php" {
		t.Errorf("request path = %q, want /advancedsearch.php", gotPath)
	}

	for _, clause := range []string{"title:(night of the)", "mediatype:(movies)", licenseFilter} {
		if !strings.Contains(gotQuery, clause) {
			t.Errorf("query %q does not contain %q", gotQuery, clause)
		}
	}

	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}

	torrent := results[0]

	if torrent.ID() != "night_of_the_living_dead" {
		t.Errorf("ID = %q", torrent.ID())
	}
	if torrent.Name() != "Night of the Living Dead" {
		t.Errorf("Name = %q", torrent.Name())
	}
	if torrent.Description() != "A classic.\nPublic domain." {
		t.Errorf("Description = %q", torrent.Description())
	}
	if torrent.Username() != "George A. Romero" {
		t.Errorf("Username = %q", torrent.Username())
	}
	if torrent.SizeGB() != 1.5 {
		t.Errorf("SizeGB = %v, want 1.5", torrent.SizeGB())
	}
	if torrent.NumFiles() != 12 {
		t.Errorf("NumFiles = %d, want 12", torrent.NumFiles())
	}
	if want := time.Date(2004, 3, 19, 0, 0, 0, 0, time.UTC); !torrent.AddedTime().Equal(want) {
		t.Errorf("AddedTime = %v, want %v", torrent.AddedTime(), want)
	}
	if want := server.URL + "/download/night_of_the_living_dead/night_of_the_living_dead_archive.torrent"; torrent.MagnetLink() != want {
		t.Errorf("MagnetLink = %q, want %q", torrent.MagnetLink(), want)
	}
	if want := server.URL + "/details/night_of_the_living_dead"; client.SiteUrl(torrent) != want {
		t.Errorf("SiteUrl = %q, want %q", client.SiteUrl(torrent), want)
	}
}

func TestSearchNoResults(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"response": {"docs": []}}`))
	}))
	defer server.Close()

	if _, err := New(server.URL, server.URL).Search(context.Background(), "nothing", 10); err == nil {
		t.Fatal("expected an error for empty search results")
	}
}

func TestSearchEmptyQuery(t *testing.T) {
	if _, err := NewDefault().Search(context.Background(), `"*:"`, 10); err == nil {
		t.Fatal("expected an error for a query with no searchable terms")
	}
}

func TestFind(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/download/some_movie/some_movie_archive.torrent" {
			w.Write([]byte("raw torrent contents"))
			return
		}

		if r.URL.Path != "/metadata/some_movie" {
			t.Errorf("request path = %q, want /metadata/some_movie", r.URL.Path)
		}

		w.Write([]byte(`{
			"item_size": 1073741824,
			"files_count": 5,
			"metadata": {
				"identifier": "some_movie",
				"mediatype": "movies",
				"title": "Some Movie",
				"description": "A movie.",
				"creator": "Someone",
				"licenseurl": "https://creativecommons.org/licenses/by/4.0/",
				"publicdate": "2004-03-19 12:30:00"
			}
		}`))
	}))
	defer server.Close()

	torrent, err := New(server.URL, server.URL).Find(context.Background(), "some_movie")
	if err != nil {
		t.Fatalf("Find: %v", err)
	}

	if torrent.ID() != "some_movie" {
		t.Errorf("ID = %q", torrent.ID())
	}
	if torrent.Name() != "Some Movie" {
		t.Errorf("Name = %q", torrent.Name())
	}
	if torrent.SizeGB() != 1.0 {
		t.Errorf("SizeGB = %v, want 1.0", torrent.SizeGB())
	}
	if torrent.NumFiles() != 5 {
		t.Errorf("NumFiles = %d, want 5", torrent.NumFiles())
	}
	if want := time.Date(2004, 3, 19, 12, 30, 0, 0, time.UTC); !torrent.AddedTime().Equal(want) {
		t.Errorf("AddedTime = %v, want %v", torrent.AddedTime(), want)
	}

	contents, err := torrent.(Torrent).TorrentFile(context.Background())
	if err != nil {
		t.Fatalf("TorrentFile: %v", err)
	}
	if string(contents) != "raw torrent contents" {
		t.Errorf("TorrentFile = %q", contents)
	}
}

func TestFindRejections(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{
			name: "unknown identifier",
			body: `{}`,
		},
		{
			name: "not a movie",
			body: `{"metadata": {"identifier": "x", "mediatype": "texts", "licenseurl": "https://creativecommons.org/licenses/by/4.0/"}}`,
		},
		{
			name: "no license",
			body: `{"metadata": {"identifier": "x", "mediatype": "movies"}}`,
		},
		{
			name: "all rights reserved license",
			body: `{"metadata": {"identifier": "x", "mediatype": "movies", "licenseurl": "https://example.com/all-rights-reserved"}}`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte(test.body))
			}))
			defer server.Close()

			if _, err := New(server.URL, server.URL).Find(context.Background(), "x"); err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}
