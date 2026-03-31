package dockerapi

import (
	"bufio"
	"encoding/binary"
	"errors"
	"io"
	"strings"
)

func ParseHistory(data []byte, tty bool) []LogLine {
	if tty {
		return parseRawBuffer(data)
	}
	return parseMuxedBuffer(data)
}

func StreamLogs(r io.Reader, tty bool, emit func(LogLine) error) error {
	if tty {
		return streamRawLogs(r, emit)
	}
	return streamMuxedLogs(r, emit)
}

func parseRawBuffer(data []byte) []LogLine {
	lines := splitNonEmptyLines(string(data))
	result := make([]LogLine, 0, len(lines))
	for _, line := range lines {
		result = append(result, LogLine{Type: "stdout", Text: line})
	}
	return result
}

func parseMuxedBuffer(data []byte) []LogLine {
	lines, ok := decodeMuxed(data)
	if ok {
		return lines
	}
	return parseRawBuffer(data)
}

func decodeMuxed(data []byte) ([]LogLine, bool) {
	if len(data) == 0 {
		return nil, true
	}

	lines := make([]LogLine, 0)
	offset := 0
	seenFrame := false

	for offset < len(data) {
		if offset+8 > len(data) {
			return nil, false
		}

		streamType := data[offset]
		size := int(binary.BigEndian.Uint32(data[offset+4 : offset+8]))
		offset += 8

		if offset+size > len(data) {
			return nil, false
		}

		seenFrame = true
		text := string(data[offset : offset+size])
		offset += size

		for _, line := range splitNonEmptyLines(text) {
			lines = append(lines, LogLine{
				Type: dockerStreamName(streamType),
				Text: line,
			})
		}
	}

	return lines, seenFrame
}

func streamRawLogs(r io.Reader, emit func(LogLine) error) error {
	reader := bufio.NewReader(r)

	for {
		line, err := reader.ReadString('\n')
		if line != "" {
			line = strings.TrimSuffix(line, "\n")
			line = strings.TrimSuffix(line, "\r")
			if strings.TrimSpace(line) != "" {
				if emitErr := emit(LogLine{Type: "stdout", Text: line}); emitErr != nil {
					return emitErr
				}
			}
		}

		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
	}
}

func streamMuxedLogs(r io.Reader, emit func(LogLine) error) error {
	var header [8]byte

	for {
		if _, err := io.ReadFull(r, header[:]); err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
				return nil
			}
			return err
		}

		size := int(binary.BigEndian.Uint32(header[4:8]))
		payload := make([]byte, size)
		if _, err := io.ReadFull(r, payload); err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
				return nil
			}
			return err
		}

		for _, line := range splitNonEmptyLines(string(payload)) {
			if err := emit(LogLine{
				Type: dockerStreamName(header[0]),
				Text: line,
			}); err != nil {
				return err
			}
		}
	}
}

func splitNonEmptyLines(text string) []string {
	parts := strings.Split(text, "\n")
	lines := make([]string, 0, len(parts))

	for _, part := range parts {
		part = strings.TrimSuffix(part, "\r")
		if strings.TrimSpace(part) == "" {
			continue
		}
		lines = append(lines, part)
	}

	return lines
}

func dockerStreamName(streamType byte) string {
	if streamType == 2 {
		return "stderr"
	}
	return "stdout"
}
