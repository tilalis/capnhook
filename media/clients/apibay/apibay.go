package apibay

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/tilalis/capnhook/media/interfaces"
)

type Apibay struct {
	apiUrl  string
	siteUrl string
}

func New(apiUrl string, siteUrl string) *Apibay {
	if apiUrl == "" {
		apiUrl = "https://apibay.org"
	}
	if siteUrl == "" {
		siteUrl = "https://thepiratebay.org"
	}

	return &Apibay{
		apiUrl:  apiUrl,
		siteUrl: siteUrl,
	}
}

func NewDefault() *Apibay {
	return New("", "")
}

func (p *Apibay) Find(ctx context.Context, id string) (interfaces.TorrentSearchResult, error) {
	body, err := p.request(ctx, fmt.Sprintf("t.php?id=%s", url.QueryEscape(id)))

	if err != nil {
		return nil, err
	}

	var torrent Torrent

	if err := json.Unmarshal(body, &torrent); err != nil {
		return nil, fmt.Errorf("unmarshal torrent response: %w", err)
	}

	if torrent.IsEmpty() {
		return nil, fmt.Errorf("torrent not found by ID: %s", id)
	}

	return torrent, nil
}

func (p *Apibay) Search(ctx context.Context, query string, limit int) ([]interfaces.TorrentSearchResult, error) {
	body, err := p.request(ctx, fmt.Sprintf("q.php?q=%s", url.QueryEscape(query)))

	if err != nil {
		return nil, err
	}

	var torrents []Torrent
	if err := json.Unmarshal(body, &torrents); err != nil {
		return nil, fmt.Errorf("unmarshal search response: %w", err)
	}

	if len(torrents) == 1 && torrents[0].IsEmpty() { 
		return nil, fmt.Errorf("no torrents found by query: %s", query)
	}

	if limit > 0 && len(torrents) > limit {
		torrents = torrents[:limit]
	}

	var torrentSearchResults []interfaces.TorrentSearchResult = make([]interfaces.TorrentSearchResult, len(torrents))
	for i, torrent := range torrents {
		torrentSearchResults[i] = torrent
	}

	return torrentSearchResults, nil
}

func (p *Apibay) SiteUrl(t interfaces.TorrentSearchResult) string {
	return fmt.Sprintf("%s/description.php?id=%s", p.siteUrl, t.ID())
}

func (p *Apibay) request(ctx context.Context, endpoint string) ([]byte, error) {
	url := fmt.Sprintf("%s/%s", p.apiUrl, endpoint)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request for %s: %w", url, err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:152.0) Gecko/20100101 Firefox/152.0")

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("error fetching the api: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)

	if err != nil {
		return nil, fmt.Errorf("read response from %s: %w", url, err)
	}

	return body, nil
}
