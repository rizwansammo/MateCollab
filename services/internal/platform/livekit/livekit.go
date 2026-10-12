// Package livekit holds the MateCollab side of the LiveKit integration. At this stage it only
// verifies the local media server is reachable; rooms, tokens and webhooks arrive with calls
// (MEDIA.md).
package livekit

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Ping checks the LiveKit HTTP endpoint answers (GET / returns 200 "OK").
func Ping(ctx context.Context, apiURL string) error {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(apiURL, "/")+"/", nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1024))
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("livekit: unexpected status %d", resp.StatusCode)
	}
	return nil
}
