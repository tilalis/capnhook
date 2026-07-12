package apibay

import (
	"encoding/json"
	"net/url"
	"time"
)

type Torrent struct {
	RawID       json.Number `json:"id"`
	RawName     string      `json:"name"`
	InfoHash    string      `json:"info_hash"`
	RawSize     json.Number `json:"size"`
	RawNumFiles json.Number `json:"num_files"`
	RawUsername string      `json:"username"`
	RawAdded    json.Number `json:"added"`
	RawDescr    string      `json:"descr"`
}

func (t Torrent) ID() string {
	return t.RawID.String()
}

func (t Torrent) Name() string {
	return t.RawName
}

func (t Torrent) SizeGB() float64 {
	sizeBytes, err := t.RawSize.Float64()

	if err != nil {
		return 0.0
	}

	return sizeBytes / float64(1<<30)
}

func (t Torrent) NumFiles() int64 {
	// swallow the error and return 0 on error
	numFiles, _ := t.RawNumFiles.Int64()
	return numFiles
}

func (t Torrent) Username() string {
	return t.RawUsername
}

func (t Torrent) AddedTime() time.Time {
	addedTs, err := t.RawAdded.Int64()
	if err != nil {
		return time.Time{}
	}
	return time.Unix(addedTs, 0)
}

func (t Torrent) Description() string {
	return t.RawDescr
}

func (t Torrent) IsEmpty() bool {
	return t.ID() == "0" && t.RawSize.String() == "0" && t.RawNumFiles.String() == "0" && t.RawUsername == ""
}

const trackers string = "&tr=udp://tracker.opentrackr.org:1337&tr=udp://open.stealth.si:80/announce&tr=udp://tracker.torrent.eu.org:451/announce&tr=udp://tracker.bittor.pw:1337/announce&tr=udp://public.popcorn-tracker.org:6969/announce&tr=udp://tracker.dler.org:6969/announce&tr=udp://exodus.desync.com:6969&tr=udp://open.demonii.com:1337/announce&tr=udp://glotorrents.pw:6969/announce&tr=udp://tracker.coppersurfer.tk:6969&tr=udp://torrent.gresille.org:80/announce&tr=udp://p4p.arenabg.com:1337&tr=udp://tracker.internetwarriors.net:1337"

func (t Torrent) MagnetLink() string {
	return "magnet:?xt=urn:btih:" + t.InfoHash + "&dn=" + url.QueryEscape(t.RawName) + trackers
}
