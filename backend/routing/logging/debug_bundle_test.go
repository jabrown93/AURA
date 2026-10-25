package routes_logging

import (
	"archive/zip"
	"aura/config"
	"aura/logging"
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestGetDebugBundle(t *testing.T) {
	// A Jellyfin server that answers the connection check
	jellyfin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"Version":"10.10.7","ServerName":"Moose's Jellyfin"}`))
	}))
	t.Cleanup(jellyfin.Close)

	folder := t.TempDir()
	previousFolder, previousConfig := logging.LogFolder, config.Current
	t.Cleanup(func() { logging.LogFolder, config.Current = previousFolder, previousConfig })
	logging.LogFolder = folder
	config.Current = config.DefaultConfig()
	config.Current.MainMediaServer = "main"
	config.Current.MediaServers = []config.Config_MediaServer{
		{ID: "main", Name: "Moose's Jellyfin", Type: "Jellyfin", Enabled: true, URL: jellyfin.URL, ApiToken: "jellyfin-token"},
		{ID: "off", Name: "Old Plex", Type: "Plex", URL: "http://10.1.1.100:32400", ApiToken: "plex-token-value"},
	}

	// The newest log files are added, the rest are over the size limit
	writeLog := func(name, text string, age time.Duration) {
		file := filepath.Join(folder, name)
		if err := os.WriteFile(file, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
		modified := time.Now().Add(-age)
		_ = os.Chtimes(file, modified, modified)
	}
	writeLog("aura.log", `{"msg":"GET `+jellyfin.URL+`/Items failed","token":"jellyfin-token","plex":"10.1.1.100","email":"me@example.com"}`+"\n", 0)
	writeLog("aura-2026-10-19.log", "yesterday\n", time.Hour)
	writeLog("aura-2026-10-18.log", strings.Repeat("x", maxDebugBundleLogSize), 2*time.Hour)
	writeLog("notes.txt", "not a log\n", 0)

	w := httptest.NewRecorder()
	GetDebugBundle(w, httptest.NewRequest(http.MethodGet, "/api/logs/debug-bundle", nil))
	if w.Code != http.StatusOK || w.Header().Get("Content-Type") != "application/zip" {
		t.Fatalf("status = %d, content type = %q, body = %s", w.Code, w.Header().Get("Content-Type"), w.Body.String())
	}

	body := w.Body.Bytes()
	zr, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]string{}
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, _ := io.ReadAll(rc)
		rc.Close()
		files[f.Name] = string(data)
	}
	for _, name := range []string{"info.txt", "config-redacted.yaml", "logs/aura.log", "logs/aura-2026-10-19.log"} {
		if _, ok := files[name]; !ok {
			t.Errorf("bundle is missing %s, has %v", name, keys(files))
		}
	}
	if len(files) != 4 {
		t.Errorf("bundle files = %v, want the 2 newest logs only", keys(files))
	}

	// The servers' types and versions are added, but not their names
	info := files["info.txt"]
	for _, want := range []string{"Server 1 (main): Jellyfin, enabled: true", "Connection: ok, version 10.10.7", "Server 2 (other): Plex, enabled: false"} {
		if !strings.Contains(info, want) {
			t.Errorf("info.txt is missing %q:\n%s", want, info)
		}
	}

	if strings.Contains(info, "Moose") {
		t.Errorf("info.txt has the server name:\n%s", info)
	}

	// Secrets, emails and addresses are replaced everywhere
	for name, text := range files {
		for _, secret := range []string{"jellyfin-token", "plex-token-value", "10.1.1.100", "me@example.com"} {
			if strings.Contains(text, secret) {
				t.Errorf("%s still has %q", name, secret)
			}
		}
	}
}

func keys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
