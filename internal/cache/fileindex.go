package cache

import (
	"database/sql"
	"fmt"
	"time"
)

// FileEntry holds the scan state for a file.
type FileEntry struct {
	FilePath  string
	ScanHash  string
	ScannedAt time.Time
}

// SetFileScanned records that a file has been scanned with the given hash.
func (c *Cache) SetFileScanned(path, hash string) error {
	_, err := c.db.Exec(`
		INSERT OR REPLACE INTO file_index (file_path, scan_hash, scanned_at)
		VALUES (?, ?, ?)`,
		path, hash, time.Now().Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("set file scanned %s: %w", path, err)
	}
	return nil
}

// GetFileHash retrieves the last scan hash for a file.
// Returns sql.ErrNoRows if the file has not been scanned.
func (c *Cache) GetFileHash(path string) (string, error) {
	var hash string
	err := c.db.QueryRow("SELECT scan_hash FROM file_index WHERE file_path = ?", path).Scan(&hash)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", err
		}
		return "", fmt.Errorf("get file hash %s: %w", path, err)
	}
	return hash, nil
}

// GetFileEntry retrieves the full file entry including scan time.
// Returns sql.ErrNoRows if the file has not been scanned.
func (c *Cache) GetFileEntry(path string) (*FileEntry, error) {
	var entry FileEntry
	var scannedAt string
	err := c.db.QueryRow(`
		SELECT file_path, scan_hash, scanned_at FROM file_index WHERE file_path = ?`,
		path).Scan(&entry.FilePath, &entry.ScanHash, &scannedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, err
		}
		return nil, fmt.Errorf("get file entry %s: %w", path, err)
	}
	entry.ScannedAt, _ = time.Parse(time.RFC3339, scannedAt)
	return &entry, nil
}

// IsFileChanged checks if a file's content has changed since last scan.
// Returns true if the file has changed or has never been scanned.
func (c *Cache) IsFileChanged(path, newHash string) (bool, error) {
	oldHash, err := c.GetFileHash(path)
	if err == sql.ErrNoRows {
		// File has never been scanned
		return true, nil
	}
	if err != nil {
		return false, err
	}
	return oldHash != newHash, nil
}

// GetAllFileEntries retrieves all file entries from the index.
func (c *Cache) GetAllFileEntries() ([]FileEntry, error) {
	rows, err := c.db.Query(`
		SELECT file_path, scan_hash, scanned_at FROM file_index ORDER BY file_path`)
	if err != nil {
		return nil, fmt.Errorf("query file entries: %w", err)
	}
	defer rows.Close()

	var entries []FileEntry
	for rows.Next() {
		var entry FileEntry
		var scannedAt string
		err := rows.Scan(&entry.FilePath, &entry.ScanHash, &scannedAt)
		if err != nil {
			return nil, fmt.Errorf("scan row: %w", err)
		}
		entry.ScannedAt, _ = time.Parse(time.RFC3339, scannedAt)
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate rows: %w", err)
	}
	return entries, nil
}

// DeleteFileEntry removes a file from the index.
func (c *Cache) DeleteFileEntry(path string) error {
	_, err := c.db.Exec("DELETE FROM file_index WHERE file_path = ?", path)
	if err != nil {
		return fmt.Errorf("delete file entry %s: %w", path, err)
	}
	return nil
}

// SetBulkFilesScanned records scan state for multiple files efficiently.
func (c *Cache) SetBulkFilesScanned(entries []FileEntry) error {
	if len(entries) == 0 {
		return nil
	}

	tx, err := c.db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}

	stmt, err := tx.Prepare(`
		INSERT OR REPLACE INTO file_index (file_path, scan_hash, scanned_at)
		VALUES (?, ?, ?)`)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("prepare statement: %w", err)
	}
	defer stmt.Close()

	for _, entry := range entries {
		scannedAt := entry.ScannedAt
		if scannedAt.IsZero() {
			scannedAt = time.Now()
		}
		_, err := stmt.Exec(entry.FilePath, entry.ScanHash, scannedAt.Format(time.RFC3339))
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("save file entry %s: %w", entry.FilePath, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}

// GetChangedFiles returns files that have changed compared to the provided hashes.
// The input is a map of file path to new hash.
// Returns a slice of paths that have changed or are new.
func (c *Cache) GetChangedFiles(fileHashes map[string]string) ([]string, error) {
	var changed []string

	for path, newHash := range fileHashes {
		isChanged, err := c.IsFileChanged(path, newHash)
		if err != nil {
			return nil, err
		}
		if isChanged {
			changed = append(changed, path)
		}
	}

	return changed, nil
}

// PruneStaleEntries removes file entries for files no longer in the provided set.
// This is useful for cleaning up entries for deleted files.
func (c *Cache) PruneStaleEntries(validPaths map[string]bool) (int, error) {
	entries, err := c.GetAllFileEntries()
	if err != nil {
		return 0, err
	}

	var pruned int
	for _, entry := range entries {
		if !validPaths[entry.FilePath] {
			if err := c.DeleteFileEntry(entry.FilePath); err != nil {
				return pruned, err
			}
			pruned++
		}
	}

	return pruned, nil
}
