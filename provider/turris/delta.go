package turris

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/vmihailenco/msgpack/v5"
)

type Delta struct {
	Operation string
	IP        net.IP
	Serial    uint16
	Timestamp time.Time
}

var deltaMapExpectedKeys = []string{
	"delta", "ip", "serial", "ts",
}

func (c *Client) decodeDelta(rawMsg []byte) (Delta, error) {
	buf := bytes.NewReader(rawMsg)
	d := msgpack.NewDecoder(buf)

	rawMap, err := d.DecodeMap()
	if err != nil {
		return Delta{}, fmt.Errorf("unable to decode delta message: %w", err)
	}

	for _, key := range deltaMapExpectedKeys {
		if _, ok := rawMap[key]; !ok {
			return Delta{}, errors.New("malformed delta message")
		}
	}

	operation := rawMap["delta"].(string)
	ip := rawMap["ip"].(string)
	serial := rawMap["serial"].(uint16)
	ts := rawMap["ts"].(uint32)

	return Delta{
		Operation: operation,
		IP:        net.ParseIP(ip),
		Serial:    serial,
		Timestamp: time.Unix(int64(ts), 0),
	}, nil
}
