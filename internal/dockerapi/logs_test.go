package dockerapi

import (
	"encoding/binary"
	"testing"
)

func TestParseMuxedBuffer(t *testing.T) {
	data := append(muxedFrame(1, "2026-03-31T10:00:00.000000000Z hello\n\n"), muxedFrame(2, "2026-03-31T10:00:01.000000000Z boom\n")...)

	lines := parseMuxedBuffer(data)
	if len(lines) != 2 {
		t.Fatalf("expected 2 log lines, got %d", len(lines))
	}

	if lines[0].Type != "stdout" || lines[0].Text != "2026-03-31T10:00:00.000000000Z hello" {
		t.Fatalf("unexpected first line: %#v", lines[0])
	}

	if lines[1].Type != "stderr" || lines[1].Text != "2026-03-31T10:00:01.000000000Z boom" {
		t.Fatalf("unexpected second line: %#v", lines[1])
	}
}

func TestParseMuxedBufferFallsBackToRaw(t *testing.T) {
	lines := parseMuxedBuffer([]byte("2026-03-31T10:00:00.000000000Z raw tty line\n"))
	if len(lines) != 1 {
		t.Fatalf("expected 1 raw line, got %d", len(lines))
	}

	if lines[0].Type != "stdout" || lines[0].Text != "2026-03-31T10:00:00.000000000Z raw tty line" {
		t.Fatalf("unexpected raw line: %#v", lines[0])
	}
}

func TestSplitNonEmptyLines(t *testing.T) {
	lines := splitNonEmptyLines("one\n\n  \ntwo\r\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}

	if lines[0] != "one" || lines[1] != "two" {
		t.Fatalf("unexpected lines: %#v", lines)
	}
}

func muxedFrame(streamType byte, payload string) []byte {
	frame := make([]byte, 8+len(payload))
	frame[0] = streamType
	binary.BigEndian.PutUint32(frame[4:8], uint32(len(payload)))
	copy(frame[8:], payload)
	return frame
}
