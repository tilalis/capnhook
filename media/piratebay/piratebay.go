package piratebay

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

type Piratebay struct {
	apiUrl  string
	siteUrl string
}

func New(apiUrl string, siteUrl string) *Piratebay {
	if apiUrl == "" {
		apiUrl = "https://apibay.org"
	}
	if siteUrl == "" {
		siteUrl = "https://thepiratebay.org"
	}

	return &Piratebay{
		apiUrl:  apiUrl,
		siteUrl: siteUrl,
	}
}

func NewDefault() *Piratebay {
	return New("", "")
}

func (p *Piratebay) Find(ctx context.Context, id string) (*Torrent, error) {
	body, err := p.request(ctx, fmt.Sprintf("t.php?id=%s", url.QueryEscape(id)))

	if err != nil {
		return nil, err
	}

	var torrent Torrent

	if err := json.Unmarshal(body, &torrent); err != nil {
		return nil, err
	}

	if torrent.IsEmpty() {
		return nil, fmt.Errorf("torrent not found by ID: %s", id)
	}

	return &torrent, nil
}

func (p *Piratebay) Search(ctx context.Context, query string) ([]Torrent, error) {
	body, err := p.request(ctx, fmt.Sprintf("q.php?q=%s", url.QueryEscape(query)))

	if err != nil {
		return nil, err
	}

	var torrents []Torrent
	if err := json.Unmarshal(body, &torrents); err != nil {
		return nil, err
	}

	if len(torrents) == 1 && torrents[0].IsEmpty() {
		return nil, fmt.Errorf("no torrents found by query: %s", query)
	}

	return torrents, nil
}

func (p *Piratebay) SiteUrl(t *Torrent) string {
	return fmt.Sprintf("%s/description.php?id=%s", p.siteUrl, t.ID)
}

func (p *Piratebay) request(ctx context.Context, endpoint string) ([]byte, error) {
	url := fmt.Sprintf("%s/%s", p.apiUrl, endpoint)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:152.0) Gecko/20100101 Firefox/152.0")

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("error fetching the api: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)

	if err != nil {
		return nil, err
	}

	return body, nil
}
