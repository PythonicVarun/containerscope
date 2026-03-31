package ws

import (
	"bufio"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

type Conn struct {
	conn      net.Conn
	reader    *bufio.Reader
	writeMu   sync.Mutex
	closeOnce sync.Once
}

func Upgrade(w http.ResponseWriter, r *http.Request) (*Conn, error) {
	if r.Method != http.MethodGet {
		return nil, fmt.Errorf("websocket requires GET")
	}
	if !headerContainsToken(r.Header, "Connection", "Upgrade") {
		return nil, fmt.Errorf("missing websocket upgrade header")
	}
	if !strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
		return nil, fmt.Errorf("missing websocket upgrade header")
	}

	key := strings.TrimSpace(r.Header.Get("Sec-WebSocket-Key"))
	if key == "" {
		return nil, fmt.Errorf("missing Sec-WebSocket-Key")
	}

	hijacker, ok := w.(http.Hijacker)
	if !ok {
		return nil, fmt.Errorf("server does not support websocket hijacking")
	}

	conn, rw, err := hijacker.Hijack()
	if err != nil {
		return nil, err
	}

	accept := websocketAccept(key)
	if _, err := rw.WriteString(
		"HTTP/1.1 101 Switching Protocols\r\n" +
			"Upgrade: websocket\r\n" +
			"Connection: Upgrade\r\n" +
			"Sec-WebSocket-Accept: " + accept + "\r\n\r\n",
	); err != nil {
		conn.Close()
		return nil, err
	}
	if err := rw.Flush(); err != nil {
		conn.Close()
		return nil, err
	}

	return &Conn{
		conn:   conn,
		reader: rw.Reader,
	}, nil
}

func (c *Conn) ReadText() (string, error) {
	for {
		opcode, payload, err := c.readFrame()
		if err != nil {
			return "", err
		}

		switch opcode {
		case 0x1:
			return string(payload), nil
		case 0x8:
			_ = c.writeFrame(0x8, nil)
			return "", io.EOF
		case 0x9:
			if err := c.writeFrame(0xA, payload); err != nil {
				return "", err
			}
		case 0xA:
			continue
		default:
			return "", fmt.Errorf("unsupported websocket opcode %d", opcode)
		}
	}
}

func (c *Conn) WriteJSON(v any) error {
	payload, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return c.writeFrame(0x1, payload)
}

func (c *Conn) Close() error {
	var closeErr error
	c.closeOnce.Do(func() {
		closeErr = c.conn.Close()
	})
	return closeErr
}

func (c *Conn) readFrame() (byte, []byte, error) {
	var header [2]byte
	if _, err := io.ReadFull(c.reader, header[:]); err != nil {
		return 0, nil, err
	}

	fin := header[0]&0x80 != 0
	if !fin {
		return 0, nil, fmt.Errorf("fragmented websocket frames are not supported")
	}

	opcode := header[0] & 0x0F
	masked := header[1]&0x80 != 0
	if !masked {
		return 0, nil, fmt.Errorf("client websocket frames must be masked")
	}

	payloadLen := int64(header[1] & 0x7F)
	switch payloadLen {
	case 126:
		var sizeBuf [2]byte
		if _, err := io.ReadFull(c.reader, sizeBuf[:]); err != nil {
			return 0, nil, err
		}
		payloadLen = int64(binary.BigEndian.Uint16(sizeBuf[:]))
	case 127:
		var sizeBuf [8]byte
		if _, err := io.ReadFull(c.reader, sizeBuf[:]); err != nil {
			return 0, nil, err
		}
		payloadLen = int64(binary.BigEndian.Uint64(sizeBuf[:]))
	}

	var maskKey [4]byte
	if _, err := io.ReadFull(c.reader, maskKey[:]); err != nil {
		return 0, nil, err
	}

	payload := make([]byte, payloadLen)
	if _, err := io.ReadFull(c.reader, payload); err != nil {
		return 0, nil, err
	}

	for i := range payload {
		payload[i] ^= maskKey[i%4]
	}

	return opcode, payload, nil
}

func (c *Conn) writeFrame(opcode byte, payload []byte) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	if err := c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second)); err != nil {
		return err
	}
	defer c.conn.SetWriteDeadline(time.Time{})

	var header []byte
	switch {
	case len(payload) < 126:
		header = []byte{0x80 | opcode, byte(len(payload))}
	case len(payload) <= 65535:
		header = make([]byte, 4)
		header[0] = 0x80 | opcode
		header[1] = 126
		binary.BigEndian.PutUint16(header[2:], uint16(len(payload)))
	default:
		header = make([]byte, 10)
		header[0] = 0x80 | opcode
		header[1] = 127
		binary.BigEndian.PutUint64(header[2:], uint64(len(payload)))
	}

	if _, err := c.conn.Write(header); err != nil {
		return err
	}
	if len(payload) == 0 {
		return nil
	}
	_, err := c.conn.Write(payload)
	return err
}

func websocketAccept(key string) string {
	sum := sha1.Sum([]byte(key + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"))
	return base64.StdEncoding.EncodeToString(sum[:])
}

func headerContainsToken(headers http.Header, name, token string) bool {
	for _, value := range headers.Values(name) {
		parts := strings.Split(value, ",")
		for _, part := range parts {
			if strings.EqualFold(strings.TrimSpace(part), token) {
				return true
			}
		}
	}
	return false
}
