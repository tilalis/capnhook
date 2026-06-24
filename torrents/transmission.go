package torrents

import (
	"context"
	url_ "net/url"

	"github.com/hekmon/transmissionrpc/v3"
)

type TransmissionClient struct {
	*transmissionrpc.Client
}

func (c *TransmissionClient) TorrentGetByID(ctx context.Context, id int64) (transmissionrpc.Torrent, error) {
	torrents, err := c.TorrentGetAllFor(ctx, []int64{id})
	if err != nil {
		return transmissionrpc.Torrent{}, err
	}
	return torrents[0], nil
}

const defaultUrl = "http://192.168.1.42:9091/transmission/rpc"

func NewTransmissionClient(url string) (*TransmissionClient, error) {
	if url == "" {
		url = defaultUrl
	}

	endpoint, err := url_.Parse(url)
	if err != nil {
		return nil, err
	}
	client, err := transmissionrpc.New(endpoint, nil)
	if err != nil {
		return nil, err
	}

	return &TransmissionClient{
		client,
	}, nil
}
