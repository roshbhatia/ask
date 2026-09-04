// Package snapshot keeps rolling terminal snapshots supplied by an external integration.
package snapshot

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/roshbhatia/ask/internal/store"
)

const (
	widest = 96 * 1024
)

// ID returns the stable identifier supplied by the active capture integration.
func ID() (string, error) {
	id := strings.TrimSpace(os.Getenv("ASK_CAPTURE_ID"))
	if id == "" {
		return "", errors.New("$ASK_CAPTURE_ID is unset, so there is no terminal snapshot to read")
	}
	if id == "." || id == ".." || filepath.Base(id) != id {
		return "", errors.New("$ASK_CAPTURE_ID must be one file-name-safe identifier")
	}
	return id, nil
}

func files(id string) (prev string, now string) {
	dir := store.Dir()
	return filepath.Join(dir, "capture-"+id+"-prev"), filepath.Join(dir, "capture-"+id+"-now")
}

// Capture rotates snapshots and stores text supplied by an external integration.
func Capture(id string, text []byte) error {
	prev, now := files(id)
	if err := os.MkdirAll(filepath.Dir(now), 0o700); err != nil {
		return err
	}
	if _, err := os.Stat(now); err == nil {
		if err := os.Rename(now, prev); err != nil {
			return err
		}
	}
	return os.WriteFile(now, text, 0o600)
}

// Last answers with what the previous command printed.
func Last(id string) ([]byte, error) {
	prevPath, nowPath := files(id)
	now, err := os.ReadFile(nowPath)
	if err != nil {
		return nil, errors.New("no terminal snapshot yet; the integration has not run")
	}
	prev, err := os.ReadFile(prevPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil, errors.New("one terminal snapshot exists; capture the next command boundary before using --last")
	}
	if err != nil {
		return nil, err
	}

	grown := Delta(lines(prev), lines(now))
	if len(grown) > 0 {
		// The prompt can change between boundaries, so the final line is always capture metadata.
		grown = grown[:len(grown)-1]
	}
	if len(grown) == 0 {
		return nil, errors.New("the previous command printed nothing")
	}
	return trim([]byte(strings.Join(grown, "\n"))), nil
}

func lines(text []byte) []string {
	cut := strings.Split(strings.ReplaceAll(string(text), "\r\n", "\n"), "\n")
	for len(cut) > 0 && strings.TrimSpace(cut[len(cut)-1]) == "" {
		cut = cut[:len(cut)-1]
	}
	return cut
}

// Delta answers with the lines that appeared between two snapshots. A rolled
// scrollback leaves no shared prefix, so it then anchors on the older snapshot's
// last line, which is the prompt the previous command was typed at.
func Delta(prev, now []string) []string {
	shared := 0
	for shared < len(prev) && shared < len(now) && prev[shared] == now[shared] {
		shared++
	}
	if shared == len(prev) {
		return now[shared:]
	}

	anchor := prev[len(prev)-1]
	for at := len(now) - 1; at >= 0; at-- {
		if now[at] == anchor {
			return now[at+1:]
		}
	}
	return now[shared:]
}

// trim keeps the tail, as the end of a failing command says why it failed.
func trim(text []byte) []byte {
	if len(text) <= widest {
		return text
	}
	cut := text[len(text)-widest:]
	if at := strings.IndexByte(string(cut), '\n'); at >= 0 {
		cut = cut[at+1:]
	}
	return append([]byte("[earlier output cut]\n"), cut...)
}
