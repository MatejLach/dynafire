package turris

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/vmihailenco/msgpack/v5"
)

type List struct {
	Version   time.Time
	Serial    uint32
	Blacklist []net.IP
	Timestamp time.Time
}

var listMapExpectedKeys = []string{
	"ts", "version", "serial", "list",
}

func (c *Client) decodeList(rawMsg []byte) (List, error) {
	buf := bytes.NewReader(rawMsg)
	d := msgpack.NewDecoder(buf)
	blacklist := make([]net.IP, 0)

	rawMap, err := d.DecodeMap()
	if err != nil {
		return List{}, fmt.Errorf("unable to decode list message: %w", err)
	}

	for _, key := range listMapExpectedKeys {
		if _, ok := rawMap[key]; !ok {
			return List{}, errors.New("malformed list message")
		}
	}

	version := rawMap["version"].(uint32)
	serial := rawMap["serial"].(uint32)
	rawBlocklist := rawMap["list"].([]interface{})
	ts := rawMap["ts"].(uint32)

	for _, ip := range rawBlocklist {
		if ipStr, ok := ip.(string); ok {
			blacklist = append(blacklist, net.ParseIP(ipStr))
		}
	}

	return List{
		Version:   time.Unix(int64(version), 0),
		Serial:    serial,
		Blacklist: blacklist,
		Timestamp: time.Unix(int64(ts), 0),
	}, nil
}
