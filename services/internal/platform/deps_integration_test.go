//go:build integration

// Integration checks for the non-database dependencies (Valkey, object storage, SMTP). Each test
// skips when its dependency is not configured, so CI can run the subset it provides.
package platform_test

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/rizwansammo/MateCollab/services/internal/platform/config"
	"github.com/rizwansammo/MateCollab/services/internal/platform/mailer"
	"github.com/rizwansammo/MateCollab/services/internal/platform/objectstore"
	"github.com/rizwansammo/MateCollab/services/internal/platform/redisx"
	"github.com/rizwansammo/MateCollab/services/internal/testkit"
)

func randomSuffix() string {
	var b [5]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

func TestValkeyRoundTrip(t *testing.T) {
	c, err := redisx.New(testkit.Require(t, testkit.EnvRedisURL))
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	ctx := context.Background()
	if err := redisx.Ping(ctx, c); err != nil {
		t.Fatalf("ping: %v", err)
	}
	key := redisx.Key("test", randomSuffix())
	if err := c.Set(ctx, key, "ok", 30*time.Second).Err(); err != nil {
		t.Fatal(err)
	}
	if v, err := c.Get(ctx, key).Result(); err != nil || v != "ok" {
		t.Fatalf("get: %q %v", v, err)
	}
	if ttl := c.TTL(ctx, key).Val(); ttl <= 0 {
		t.Fatalf("expected TTL, got %v", ttl)
	}
}

func TestObjectStorageRoundTripAndPresignedDownload(t *testing.T) {
	endpoint := testkit.Require(t, testkit.EnvS3Endpoint)
	cfg := config.StorageConfig{
		Endpoint: endpoint, PublicEndpoint: endpoint, Region: "us-east-1", ForcePathStyle: true,
		Bucket:          "mc-it-" + randomSuffix(),
		AccessKeyID:     testkit.Require(t, testkit.EnvS3AccessKey),
		SecretAccessKey: testkit.Require(t, testkit.EnvS3SecretKey),
	}
	s, err := objectstore.NewS3(cfg)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	res, err := s.Init(ctx, []string{"http://localhost:8080"})
	if err != nil || !res.BucketCreated {
		t.Fatalf("init: %+v %v", res, err)
	}
	key := "orgs/test/files/" + randomSuffix() + "/original"
	body := "hello object storage"
	if err := s.Put(ctx, key, strings.NewReader(body), int64(len(body)), "text/plain"); err != nil {
		t.Fatalf("put: %v", err)
	}
	info, err := s.Head(ctx, key)
	if err != nil || info.Size != int64(len(body)) {
		t.Fatalf("head: %+v %v", info, err)
	}
	signed, err := s.PresignGet(ctx, key, time.Minute, objectstore.DownloadOptions{Filename: "notes v1.txt", ContentType: "text/plain"})
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.Get(signed)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 || string(got) != body {
		t.Fatalf("presigned GET: %d %q", resp.StatusCode, got)
	}
	if cd := resp.Header.Get("Content-Disposition"); !strings.HasPrefix(cd, "attachment;") {
		t.Fatalf("downloads must be attachments, got %q", cd)
	}
	if err := s.Delete(ctx, key); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Head(ctx, key); err != objectstore.ErrNotFound {
		t.Fatalf("deleted object still present: %v", err)
	}
}

func TestMailerDeliversToMailpit(t *testing.T) {
	api := testkit.Require(t, testkit.EnvMailpitAPI)
	port, _ := strconv.Atoi(testkit.Require(t, testkit.EnvSMTPPort))
	m, err := mailer.New(config.SMTPConfig{Host: testkit.Require(t, testkit.EnvSMTPHost), Port: port, TLS: "none", From: "MateCollab <no-reply@localhost>"})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	subject := "integration " + randomSuffix()
	if err := m.Send(ctx, mailer.Message{To: []string{"dev@example.test"}, Subject: subject, Text: "plain", HTML: "<p>html</p>"}); err != nil {
		t.Fatalf("send: %v", err)
	}
	if err := m.Send(ctx, mailer.Message{To: []string{"x@example.test"}, Subject: "bad\r\nBcc: victim@example.test", Text: "x"}); err == nil {
		t.Fatal("header injection must be rejected")
	}

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(fmt.Sprintf("%s/api/v1/search?query=%s", strings.TrimRight(api, "/"), url.QueryEscape(`subject:"`+subject+`"`)))
		if err == nil {
			var out struct {
				MessagesCount int `json:"messages_count"`
			}
			_ = json.NewDecoder(resp.Body).Decode(&out)
			resp.Body.Close()
			if out.MessagesCount == 1 {
				return
			}
		}
		time.Sleep(250 * time.Millisecond)
	}
	t.Fatal("message did not arrive in Mailpit")
}
