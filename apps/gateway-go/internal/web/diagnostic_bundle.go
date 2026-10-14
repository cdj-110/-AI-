package web

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"sort"
	"strings"
	"time"

	"weikong-iot-platform/apps/gateway-go/internal/config"
)

const diagnosticFileLimit = 256 * 1024

type diagnosticManifestEntry struct {
	Name   string `json:"name"`
	Size   int    `json:"size"`
	SHA256 string `json:"sha256"`
}

func (s *Server) diagnosticBundle(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	entries := map[string][]byte{}
	addJSON := func(name string, value interface{}) {
		if raw, err := json.MarshalIndent(value, "", "  "); err == nil {
			entries[name] = append(raw, '\n')
		}
	}
	build := map[string]interface{}{"goVersion": runtime.Version(), "goos": runtime.GOOS, "goarch": runtime.GOARCH, "generatedAt": time.Now()}
	if info, ok := debug.ReadBuildInfo(); ok {
		build["module"] = info.Main.Path
		build["version"] = info.Main.Version
		build["settings"] = info.Settings
	}
	addJSON("system/build.json", build)
	if s.store != nil {
		addJSON("system/status.json", s.store.CompactSnapshot())
		addJSON("system/health.json", s.store.Health(20*time.Second))
	}
	if cfg, err := config.Load(s.configPath); err == nil {
		addJSON("configuration/config.redacted.json", config.RedactSecrets(cfg))
	} else {
		entries["errors/config.txt"] = []byte(err.Error() + "\n")
	}
	if cfg, err := loadTimeSyncConfig(s.timeSyncConfigPath()); err == nil {
		addJSON("system/time-sync.json", readTimeSyncStatus(cfg))
	}
	addJSON("system/ssh.json", readSSHAccessStatus(sshInitScriptCandidates, sshProcessRunning))
	if s.audit != nil {
		if events, err := s.audit.recent(200); err == nil {
			addJSON("audit/recent.json", events)
		}
	}
	addJSON("storage/files.json", diagnosticStorageFiles(filepath.Dir(s.configPath)))
	for _, item := range []struct{ name, path string }{
		{"system/meminfo.txt", "/proc/meminfo"}, {"system/cpuinfo.txt", "/proc/cpuinfo"},
		{"system/uptime.txt", "/proc/uptime"}, {"system/mounts.txt", "/proc/mounts"},
	} {
		if content, err := readDiagnosticFile(item.path); err == nil {
			entries[item.name] = content
		}
	}

	names := make([]string, 0, len(entries))
	for name := range entries {
		names = append(names, name)
	}
	sort.Strings(names)
	manifest := make([]diagnosticManifestEntry, 0, len(names))
	for _, name := range names {
		sum := sha256.Sum256(entries[name])
		manifest = append(manifest, diagnosticManifestEntry{Name: name, Size: len(entries[name]), SHA256: hex.EncodeToString(sum[:])})
	}
	addJSON("manifest.json", map[string]interface{}{"format": "weikong-diagnostic-v1", "entries": manifest})
	names = append(names, "manifest.json")

	filename := "weikong-diagnostic-" + time.Now().Format("20060102-150405") + ".zip"
	writer.Header().Set("Content-Type", "application/zip")
	writer.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	writer.Header().Set("Cache-Control", "no-store")
	archive := zip.NewWriter(writer)
	for _, name := range names {
		file, err := archive.Create(name)
		if err != nil {
			continue
		}
		_, _ = file.Write(entries[name])
	}
	_ = archive.Close()
	s.appendAudit(auditEvent{Username: currentUsername(s, request), RemoteIP: requestRemoteIP(request), Method: request.Method, Path: request.URL.Path, Status: http.StatusOK, Result: "success", Detail: "diagnostic bundle downloaded"})
}

func readDiagnosticFile(path string) ([]byte, error) {
	content, err := os.ReadFile(path)
	if len(content) > diagnosticFileLimit {
		content = content[:diagnosticFileLimit]
	}
	return content, err
}

type diagnosticStorageFile struct {
	Path    string    `json:"path"`
	Size    int64     `json:"size"`
	ModTime time.Time `json:"modifiedAt"`
}

func diagnosticStorageFiles(root string) []diagnosticStorageFile {
	var result []diagnosticStorageFile
	_ = filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || path == root {
			return nil
		}
		relative, relErr := filepath.Rel(root, path)
		if relErr != nil || strings.Count(filepath.ToSlash(relative), "/") > 2 {
			if entry != nil && entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry == nil || entry.IsDir() {
			return nil
		}
		name := strings.ToLower(entry.Name())
		if !strings.HasSuffix(name, ".db") && !strings.HasSuffix(name, ".jsonl") && !strings.HasSuffix(name, ".json") {
			return nil
		}
		if info, infoErr := entry.Info(); infoErr == nil {
			result = append(result, diagnosticStorageFile{Path: filepath.ToSlash(relative), Size: info.Size(), ModTime: info.ModTime()})
		}
		return nil
	})
	sort.Slice(result, func(i, j int) bool { return result[i].Path < result[j].Path })
	return result
}
