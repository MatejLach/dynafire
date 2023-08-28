package turris

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	zmq "github.com/pebbe/zmq4"
)

const (
	Url     = "sentinel.turris.cz"
	Port    = 7087
	CertUrl = "https://repo.turris.cz/sentinel/dynfw.pub"
)

type Client struct {
	zmqClient           *zmq.Socket
	zmqClientPrivateKey string
	zmqClientPublicKey  string
	zmqServerPublicKey  string
	zmqServerUrl        string
	zmqServerPort       int
	ListChan            chan List
	DeltaChan           chan Delta
}

func NewClient(zmqServerUrl string, zmqServerPort int) (*Client, error) {
	zmqCtx, err := zmq.NewContext()
	if err != nil {
		return nil, err
	}

	zmqClient, err := zmqCtx.NewSocket(zmq.SUB)
	if err != nil {
		return nil, err
	}

	zmqServerPubKey, err := getServerPubKey(CertUrl)
	if err != nil {
		return nil, err
	}

	zmqClientPubKey, zmqClientPrivateKey, err := zmq.NewCurveKeypair()
	if err != nil {
		return nil, err
	}

	err = zmqClient.SetSubscribe("dynfw/")
	if err != nil {
		return nil, err
	}

	return &Client{
		zmqClient:           zmqClient,
		zmqClientPrivateKey: zmqClientPrivateKey,
		zmqClientPublicKey:  zmqClientPubKey,
		zmqServerPublicKey:  zmqServerPubKey,
		zmqServerUrl:        zmqServerUrl,
		zmqServerPort:       zmqServerPort,
		ListChan:            make(chan List),
		DeltaChan:           make(chan Delta),
	}, nil
}

// Close is called automatically when you cancel the context passed in to RequestMessages
// Manual invocation of Close should be done only If you passed in context.Background()
func (c *Client) Close() {
	if c.zmqClient == nil {
		slog.Warn("ZMQ client not initialised, nothing to close")
	}

	err := c.zmqClient.SetUnsubscribe("dynfw/")
	if err != nil {
		slog.Error("unable to unsubscribe from Turris dynfw/ topic", "details", err)
	}

	err = c.zmqClient.Close()
	if err != nil {
		slog.Error("unable to close ZMQ client", "details", err)
	}

	close(c.DeltaChan)
	close(c.ListChan)
}

func (c *Client) Connect() error {
	err := c.zmqClient.ClientAuthCurve(c.zmqServerPublicKey, c.zmqClientPublicKey, c.zmqClientPrivateKey)
	if err != nil {
		return err
	}

	err = c.zmqClient.Connect(fmt.Sprintf("tcp://%s:%d", c.zmqServerUrl, c.zmqServerPort))
	if err != nil {
		return err
	}

	recvTestMsg, err := c.zmqClient.RecvMessageBytes(0)
	if err != nil {
		return fmt.Errorf("failed to verify connection, failed to receive dynfw/ test message: %w", err)
	}

	if len(recvTestMsg) == 0 {
		return errors.New("failed to verify connection, dynfw/ test message has no data")
	}

	return nil
}

func (c *Client) RequestMessages(ctx context.Context) {
	var previousDeltaSerial uint32
	refreshList := true // upon launch, initialize the list

	for {
		payloadB, err := c.zmqClient.RecvMessageBytes(0)
		if err != nil {
			slog.Error("unable to receive dynfw message", "details", err)
			return
		}

		if len(payloadB) != 2 {
			slog.Error("malformed dynfw message")
			return
		}

		switch string(payloadB[0]) {
		case "dynfw/event":
			continue
		case "dynfw/delta":
			if refreshList {
				continue
			}

			dRes, err := c.decodeDelta(payloadB[1])
			if err != nil {
				slog.Warn("unable to decode delta message", "details", err)
				continue
			}

			if !serialOk(previousDeltaSerial, dRes.Serial) {
				refreshList = true
				previousDeltaSerial = 0
			}

			previousDeltaSerial = dRes.Serial
			c.DeltaChan <- dRes
		case "dynfw/list":
			if refreshList {
				lRes, err := c.decodeList(payloadB[1])
				if err != nil {
					slog.Warn("unable to decode list message", "details", err)
					continue
				}

				refreshList = false
				c.ListChan <- lRes
			} else {
				continue
			}
		}

		select {
		case <-ctx.Done():
			c.Close()
			return
		default:
			continue
		}
	}
}

func serialOk(oldSerial, currentSerial uint32) bool {
	if (oldSerial+1 == currentSerial) || oldSerial == 0 {
		return true
	}

	return false
}

func getServerPubKey(certDlUri string) (string, error) {
	resp, err := http.Get(certDlUri)
	defer func(resp *http.Response) {
		if resp == nil {
			return
		}

		err := resp.Body.Close()
		if err != nil {
			slog.Error("unable to close HTTP response body", "details", err)
			return
		}
	}(resp)

	if err != nil {
		return "", err
	}

	certB, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	keyStartIdx := bytes.IndexByte(certB, '"')
	keyEndIdx := bytes.LastIndexByte(certB, '"')

	return string(certB[keyStartIdx+1 : keyEndIdx]), nil
}
