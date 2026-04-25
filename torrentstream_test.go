package main

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"
)

func TestListIDsReturnsTorrentMapKeys(t *testing.T) {
	app := &App{
		torrents: map[string]*T{
			"info-hash": {ID: "torrent-id"},
		},
	}

	got := app.ListIDs()
	want := []string{"info-hash"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ListIDs() = %v, want %v", got, want)
	}
}

func TestIdleForRequiresNoConnectionsAndExpiredActivity(t *testing.T) {
	torrent := &T{
		Conn:     0,
		Activity: time.Now().Add(-(torrentIdleTimeout + time.Minute)),
	}
	if !torrent.IdleFor(torrentIdleTimeout) {
		t.Fatal("expected torrent to be idle after timeout with no connections")
	}

	torrent.Conn = 1
	if torrent.IdleFor(torrentIdleTimeout) {
		t.Fatal("expected active connection to prevent idle cleanup")
	}

	torrent.Conn = 0
	torrent.Activity = time.Now()
	if torrent.IdleFor(torrentIdleTimeout) {
		t.Fatal("expected recent activity to prevent idle cleanup")
	}
}

func TestTouchRefreshesTorrentActivity(t *testing.T) {
	const magnetURL = "/touch?magnet:?xt=urn:btih:0123456789abcdef0123456789abcdef01234567&dn=test"

	req := httptest.NewRequest(http.MethodPost, magnetURL, nil)
	_, magnet, err := processMagnetURI(req)
	if err != nil {
		t.Fatal(err)
	}

	oldApp := app
	t.Cleanup(func() { app = oldApp })

	tracked := &T{Activity: time.Now().Add(-time.Hour)}
	app = &App{
		torrents: map[string]*T{
			magnet.InfoHash.String(): tracked,
		},
	}

	rec := httptest.NewRecorder()
	touch(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("touch status = %d, want %d", rec.Code, http.StatusNoContent)
	}
	if time.Since(tracked.Activity) > time.Second {
		t.Fatal("expected touch to refresh torrent activity")
	}
}
