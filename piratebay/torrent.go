package piratebay

import (
	"encoding/json"
	"time"
)

type Torrent struct {
	Id       json.Number `json:"id"`
	Name     string      `json:"name"`
	InfoHash string      `json:"info_hash"`
	Size     json.Number `json:"size"`
	NumFiles json.Number `json:"num_files"`
	Username string      `json:"username"`
	Added    json.Number `json:"added"`
	Descr    string      `json:"descr"`
	siteUrl  string
}

func (t Torrent) SizeGB() float64 {
	sizeBytes, err := t.Size.Float64()

	if err != nil {
		return 0.0
	}

	return sizeBytes / float64(1<<30)
}

func (t Torrent) AddedTime() time.Time {
	addedTs, err := t.Added.Int64()
	if err != nil {
		return time.Time{}
	}
	return time.Unix(addedTs, 0)
}

const trackers string = "&tr=udp://tracker.opentrackr.org:1337&tr=udp://open.stealth.si:80/announce&tr=udp://tracker.torrent.eu.org:451/announce&tr=udp://tracker.bittor.pw:1337/announce&tr=udp://public.popcorn-tracker.org:6969/announce&tr=udp://tracker.dler.org:6969/announce&tr=udp://exodus.desync.com:6969&tr=udp://open.demonii.com:1337/announce&tr=udp://glotorrents.pw:6969/announce&tr=udp://tracker.coppersurfer.tk:6969&tr=udp://torrent.gresille.org:80/announce&tr=udp://p4p.arenabg.com:1337&tr=udp://tracker.internetwarriors.net:1337"

func (t Torrent) MagnetLink() string {
	return "magnet:?xt=urn:btih:" + t.InfoHash + "&dn=" + t.Name + trackers
}
