package history

import (
	"bufio"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	backupManifestSuffix = ".manifest.json"
	encryptedBackupMagic = "WKHBAK1\n"
	backupChunkSize      = 1024 * 1024
)

type BackupInfo struct {
	ID        string `json:"id"`
	FileName  string `json:"fileName"`
	Path      string `json:"-"`
	CreatedAt string `json:"createdAt"`
	Bytes     int64  `json:"bytes"`
	SHA256    string `json:"sha256"`
	Encrypted bool   `json:"encrypted"`
	Integrity string `json:"integrity"`
	Schema    int    `json:"schemaVersion"`
}

type IntegrityResult struct {
	Mode      string `json:"mode"`
	Result    string `json:"result"`
	OK        bool   `json:"ok"`
	CheckedAt string `json:"checkedAt"`
	Duration  int64  `json:"durationMs"`
}

func (s *Store) CheckIntegrity(full bool) (IntegrityResult, error) {
	started := time.Now()
	result := IntegrityResult{Mode: "quick", CheckedAt: started.Format(time.RFC3339Nano)}
	if full {
		result.Mode = "full"
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.isAvailable() {
		return result, ErrDisabled
	}
	if err := s.ensureDatabaseLocked(false); err != nil {
		return result, err
	}
	if s.db == nil {
		result.Result, result.OK = "empty", true
		return result, nil
	}
	pragma := `PRAGMA quick_check;`
	if full {
		pragma = `PRAGMA integrity_check;`
	}
	rows, err := s.db.Query(pragma)
	if err != nil {
		return result, err
	}
	defer rows.Close()
	parts := make([]string, 0, 4)
	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err != nil {
			return result, err
		}
		parts = append(parts, value)
		if len(parts) >= 20 {
			break
		}
	}
	result.Result = strings.Join(parts, "; ")
	result.OK = len(parts) == 1 && strings.EqualFold(strings.TrimSpace(parts[0]), "ok")
	result.Duration = time.Since(started).Milliseconds()
	if !result.OK {
		return result, fmt.Errorf("sqlite %s integrity check failed: %s", result.Mode, result.Result)
	}
	return result, rows.Err()
}

func (s *Store) CreateBackup(destinationDir string, keep int, encryptionKey []byte) (BackupInfo, error) {
	if destinationDir == "" {
		return BackupInfo{}, errors.New("backup destination is empty")
	}
	if keep < 1 {
		keep = 3
	}
	if err := os.MkdirAll(destinationDir, 0700); err != nil {
		return BackupInfo{}, err
	}
	now := time.Now()
	id := "history-backup-" + now.Format("20060102-150405.000")
	stage := filepath.Join(destinationDir, "."+id+".tmp")
	_ = os.RemoveAll(stage)
	if err := os.MkdirAll(stage, 0700); err != nil {
		return BackupInfo{}, err
	}
	defer os.RemoveAll(stage)
	snapshot, err := s.BackupTo(stage)
	if err != nil {
		return BackupInfo{}, err
	}
	fileName := id + ".db"
	finalPath := filepath.Join(destinationDir, fileName)
	encrypted := len(encryptionKey) > 0
	if encrypted {
		fileName += ".enc"
		finalPath += ".enc"
		if err := encryptFile(snapshot, finalPath, encryptionKey); err != nil {
			_ = os.Remove(finalPath)
			return BackupInfo{}, err
		}
	} else if err := os.Rename(snapshot, finalPath); err != nil {
		return BackupInfo{}, err
	}
	digest, bytes, err := fileDigest(finalPath)
	if err != nil {
		_ = os.Remove(finalPath)
		return BackupInfo{}, err
	}
	info := BackupInfo{ID: id, FileName: fileName, Path: finalPath, CreatedAt: now.Format(time.RFC3339Nano), Bytes: bytes, SHA256: digest, Encrypted: encrypted, Integrity: "ok", Schema: 1}
	manifest, _ := json.MarshalIndent(info, "", "  ")
	if err := os.WriteFile(filepath.Join(destinationDir, id+backupManifestSuffix), manifest, 0600); err != nil {
		_ = os.Remove(finalPath)
		return BackupInfo{}, err
	}
	if err := rotateBackups(destinationDir, keep); err != nil {
		return info, err
	}
	return info, nil
}

func ListBackups(destinationDir string) ([]BackupInfo, error) {
	entries, err := os.ReadDir(destinationDir)
	if os.IsNotExist(err) {
		return []BackupInfo{}, nil
	}
	if err != nil {
		return nil, err
	}
	result := make([]BackupInfo, 0)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), backupManifestSuffix) {
			continue
		}
		data, err := os.ReadFile(filepath.Join(destinationDir, entry.Name()))
		if err != nil {
			continue
		}
		var item BackupInfo
		if json.Unmarshal(data, &item) != nil || item.ID == "" || item.FileName == "" {
			continue
		}
		item.Path = filepath.Join(destinationDir, filepath.Base(item.FileName))
		if stat, err := os.Stat(item.Path); err == nil {
			item.Bytes = stat.Size()
			result = append(result, item)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].CreatedAt > result[j].CreatedAt })
	return result, nil
}

func DeleteBackup(destinationDir, id string) error {
	item, err := findBackup(destinationDir, id)
	if err != nil {
		return err
	}
	if err := os.Remove(item.Path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return os.Remove(filepath.Join(destinationDir, item.ID+backupManifestSuffix))
}

func (s *Store) RestoreBackup(destinationDir, id string, encryptionKey []byte) (string, error) {
	item, err := findBackup(destinationDir, id)
	if err != nil {
		return "", err
	}
	digest, _, err := fileDigest(item.Path)
	if err != nil || !strings.EqualFold(digest, item.SHA256) {
		return "", fmt.Errorf("backup checksum verification failed")
	}
	if err := os.MkdirAll(s.dir, 0700); err != nil {
		return "", err
	}
	stage := DatabasePath(s.dir) + ".restore-tmp"
	_ = os.Remove(stage)
	if item.Encrypted {
		if len(encryptionKey) == 0 {
			return "", errors.New("encrypted backup key is unavailable")
		}
		if err := decryptFile(item.Path, stage, encryptionKey); err != nil {
			_ = os.Remove(stage)
			return "", err
		}
	} else if err := copyFile(item.Path, stage); err != nil {
		return "", err
	}
	defer os.Remove(stage)
	if err := verifySQLiteFile(stage); err != nil {
		return "", err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db != nil {
		_, _ = s.db.Exec(`PRAGMA wal_checkpoint(TRUNCATE);`)
		_ = s.db.Close()
		s.db = nil
	}
	destination := DatabasePath(s.dir)
	rollback := destination + ".before-restore-" + time.Now().Format("20060102-150405")
	if _, err := os.Stat(destination); err == nil {
		if err := os.Rename(destination, rollback); err != nil {
			return "", err
		}
	}
	_ = os.Remove(destination + "-wal")
	_ = os.Remove(destination + "-shm")
	if err := os.Rename(stage, destination); err != nil {
		_ = os.Rename(rollback, destination)
		return "", err
	}
	integrityChecked.Delete(destination)
	if err := s.ensureDatabaseLocked(false); err != nil {
		_ = os.Remove(destination)
		_ = os.Rename(rollback, destination)
		integrityChecked.Delete(destination)
		return "", err
	}
	return rollback, nil
}

// RollbackRestore puts back the pre-restore database when the application
// cannot reopen the restored snapshot. The failed restored file is quarantined
// for diagnostics instead of being silently discarded.
func (s *Store) RollbackRestore(rollback string) error {
	if filepath.Dir(filepath.Clean(rollback)) != filepath.Dir(filepath.Clean(DatabasePath(s.dir))) || !strings.Contains(filepath.Base(rollback), ".before-restore-") {
		return errors.New("invalid restore rollback path")
	}
	if err := verifySQLiteFile(rollback); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db != nil {
		_ = s.db.Close()
		s.db = nil
	}
	destination := DatabasePath(s.dir)
	failed := destination + ".failed-restore-" + time.Now().Format("20060102-150405")
	if _, err := os.Stat(destination); err == nil {
		if err := os.Rename(destination, failed); err != nil {
			return err
		}
	}
	_ = os.Remove(destination + "-wal")
	_ = os.Remove(destination + "-shm")
	if err := os.Rename(rollback, destination); err != nil {
		_ = os.Rename(failed, destination)
		return err
	}
	integrityChecked.Delete(destination)
	return s.ensureDatabaseLocked(false)
}

func (s *Store) Vacuum() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.isAvailable() {
		return ErrDisabled
	}
	if err := s.ensureDatabaseLocked(false); err != nil || s.db == nil {
		return err
	}
	if _, err := s.db.Exec(`PRAGMA wal_checkpoint(TRUNCATE);`); err != nil {
		return err
	}
	_, err := s.db.Exec(`VACUUM;`)
	return err
}

func findBackup(dir, id string) (BackupInfo, error) {
	id = strings.TrimSpace(id)
	if id == "" || filepath.Base(id) != id {
		return BackupInfo{}, errors.New("invalid backup id")
	}
	items, err := ListBackups(dir)
	if err != nil {
		return BackupInfo{}, err
	}
	for _, item := range items {
		if item.ID == id {
			return item, nil
		}
	}
	return BackupInfo{}, os.ErrNotExist
}

func rotateBackups(dir string, keep int) error {
	items, err := ListBackups(dir)
	if err != nil {
		return err
	}
	for _, item := range items[minimumInt(keep, len(items)):] {
		if err := DeleteBackup(dir, item.ID); err != nil {
			return err
		}
	}
	return nil
}

func fileDigest(path string) (string, int64, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer file.Close()
	hash := sha256.New()
	bytes, err := io.Copy(hash, file)
	return hex.EncodeToString(hash.Sum(nil)), bytes, err
}

func verifySQLiteFile(path string) error {
	db, err := sql.Open("sqlite3", path+"?_busy_timeout=5000&_query_only=1")
	if err != nil {
		return err
	}
	defer db.Close()
	var result string
	if err := db.QueryRow(`PRAGMA quick_check;`).Scan(&result); err != nil {
		return err
	}
	if !strings.EqualFold(strings.TrimSpace(result), "ok") {
		return fmt.Errorf("backup sqlite quick_check failed: %s", result)
	}
	return nil
}

func encryptFile(source, destination string, key []byte) error {
	block, err := aes.NewCipher(key)
	if err != nil {
		return err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := os.OpenFile(destination, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer output.Close()
	writer := bufio.NewWriter(output)
	if _, err := writer.WriteString(encryptedBackupMagic); err != nil {
		return err
	}
	prefix := make([]byte, 8)
	if _, err := rand.Read(prefix); err != nil {
		return err
	}
	if _, err := writer.Write(prefix); err != nil {
		return err
	}
	buffer := make([]byte, backupChunkSize)
	for index := uint32(0); ; index++ {
		count, readErr := input.Read(buffer)
		if count > 0 {
			nonce := append(append([]byte{}, prefix...), byte(index>>24), byte(index>>16), byte(index>>8), byte(index))
			sealed := aead.Seal(nil, nonce, buffer[:count], nil)
			if err := binary.Write(writer, binary.BigEndian, uint32(len(sealed))); err != nil {
				return err
			}
			if _, err := writer.Write(sealed); err != nil {
				return err
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return readErr
		}
	}
	if err := binary.Write(writer, binary.BigEndian, uint32(0)); err != nil {
		return err
	}
	return writer.Flush()
}

func decryptFile(source, destination string, key []byte) error {
	block, err := aes.NewCipher(key)
	if err != nil {
		return err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	reader := bufio.NewReader(input)
	magic := make([]byte, len(encryptedBackupMagic))
	if _, err := io.ReadFull(reader, magic); err != nil || string(magic) != encryptedBackupMagic {
		return errors.New("invalid encrypted backup format")
	}
	prefix := make([]byte, 8)
	if _, err := io.ReadFull(reader, prefix); err != nil {
		return err
	}
	output, err := os.OpenFile(destination, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer output.Close()
	for index := uint32(0); ; index++ {
		var size uint32
		if err := binary.Read(reader, binary.BigEndian, &size); err != nil {
			return err
		}
		if size == 0 {
			return output.Sync()
		}
		if size > backupChunkSize+uint32(aead.Overhead()) {
			return errors.New("encrypted backup chunk is invalid")
		}
		sealed := make([]byte, size)
		if _, err := io.ReadFull(reader, sealed); err != nil {
			return err
		}
		nonce := append(append([]byte{}, prefix...), byte(index>>24), byte(index>>16), byte(index>>8), byte(index))
		plain, err := aead.Open(nil, nonce, sealed, nil)
		if err != nil {
			return fmt.Errorf("encrypted backup authentication failed: %w", err)
		}
		if _, err := output.Write(plain); err != nil {
			return err
		}
	}
}

func copyFile(source, destination string) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := os.OpenFile(destination, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(output, input)
	syncErr := output.Sync()
	closeErr := output.Close()
	if copyErr != nil {
		return copyErr
	}
	if syncErr != nil {
		return syncErr
	}
	return closeErr
}

func minimumInt(left, right int) int {
	if left < right {
		return left
	}
	return right
}
