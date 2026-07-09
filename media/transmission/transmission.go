package transmission

import (
	"context"
	"errors"
	"fmt"
	"net/url"

	"github.com/hekmon/transmissionrpc/v3"
)

type Transmission struct {
	*transmissionrpc.Client
}

func (c *Transmission) TorrentGetByID(ctx context.Context, id int64) (transmissionrpc.Torrent, error) {
	torrents, err := c.TorrentGetAllFor(ctx, []int64{id})
	if err != nil {
		return transmissionrpc.Torrent{}, fmt.Errorf("get torrent %d: %w", id, err)
	}
	if len(torrents) == 0 {
		return transmissionrpc.Torrent{}, errors.New("no torrents found")
	}
	return torrents[0], nil
}

const defaultUrl = "http://192.168.1.42:9091/transmission/rpc"

func NewTransmissionClient(rawURL string) (*Transmission, error) {
	if rawURL == "" {
		rawURL = defaultUrl
	}

	endpoint, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("parse transmission url %q: %w", rawURL, err)
	}
	client, err := transmissionrpc.New(endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("create transmission client: %w", err)
	}

	return &Transmission{
		client,
	}, nil
}
