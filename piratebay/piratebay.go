package piratebay

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

type Piratebay struct {
	Url string
}

type Torrent struct {
	Id 		 string `json:"id"`
	Name     string `json:"name"`
	InfoHash string `json:"info_hash"`
	Size     string `json:"size"`
	NumFiles string `json:"num_files"`
	Username string `json:"username"`
	Added    string `json:"added"`
}

func (t Torrent) SizeGB() float64 {
	sizeBytes, err := strconv.Atoi(t.Size)

	if err != nil {
		return 0.0
	}

	return float64(sizeBytes) / float64(1 << 30)
}

func (t Torrent) AddedTime() time.Time {
	addedTs, err := strconv.Atoi(t.Added)
	if err != nil {
		return time.Time{}
	}
	return time.Unix(int64(addedTs), 0)
}

const trackers string = "&tr=udp://tracker.opentrackr.org:1337&tr=udp://open.stealth.si:80/announce&tr=udp://tracker.torrent.eu.org:451/announce&tr=udp://tracker.bittor.pw:1337/announce&tr=udp://public.popcorn-tracker.org:6969/announce&tr=udp://tracker.dler.org:6969/announce&tr=udp://exodus.desync.com:6969&tr=udp://open.demonii.com:1337/announce&tr=udp://glotorrents.pw:6969/announce&tr=udp://tracker.coppersurfer.tk:6969&tr=udp://torrent.gresille.org:80/announce&tr=udp://p4p.arenabg.com:1337&tr=udp://tracker.internetwarriors.net:1337"

func (t Torrent) MagnetLink() string {
	return "magnet:?xt=urn:btih:" + t.InfoHash + "&dn=" + t.Name + trackers
}

func (p *Piratebay) Search(query string) ([]Torrent, error) {
	url := p.Url + "/q.php?q=" + url.QueryEscape(query)

	req, err := http.NewRequest(http.MethodGet, url, nil)
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

	body, err := io.ReadAll(resp.Body)

	if err != nil {
		return nil, err
	}

	var torrents []Torrent
	if err := json.Unmarshal(body, &torrents); err != nil {
		return nil, err
	}

	return torrents, nil
}

func New(url string) *Piratebay {
	if url == "" {
		url = "https://apibay.org"
	}
	return &Piratebay{
		Url: url,
	}
}
