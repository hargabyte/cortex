package store

import (
	"database/sql"
	"fmt"
	"time"
)

// SetFileScanned records that a file has been scanned with the given hash.
func (s *Store) SetFileScanned(path, hash string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.Exec(`
        REPLACE INTO file_index (file_path, scan_hash, scanned_at)
        VALUES (?, ?, ?)`, path, hash, now)
	if err != nil {
		return fmt.Errorf("set file scanned %s: %w", path, err)
	}
	return nil
}

// SetFilesScannedBulk records scan state for multiple files efficiently.
func (s *Store) SetFilesScannedBulk(entries []*FileIndex) error {
	if len(entries) == 0 {
		return nil
	}

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
        REPLACE INTO file_index (file_path, scan_hash, scanned_at)
        VALUES (?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare statement: %w", err)
	}
	defer stmt.Close()

	now := time.Now().UTC().Format(time.RFC3339)
	for _, entry := range entries {
		scannedAt := now
		if !entry.ScannedAt.IsZero() {
			scannedAt = entry.ScannedAt.Format(time.RFC3339)
		}
		_, err := stmt.Exec(entry.FilePath, entry.ScanHash, scannedAt)
		if err != nil {
			return fmt.Errorf("save file entry %s: %w", entry.FilePath, err)
		}
	}

	return tx.Commit()
}

// GetFileHash retrieves the last scan hash for a file.
// Returns sql.ErrNoRows if the file has not been scanned.
func (s *Store) GetFileHash(path string) (string, error) {
	var hash string
	err := s.db.QueryRow("SELECT scan_hash FROM file_index WHERE file_path = ?", path).Scan(&hash)
	if err != nil {
		return "", err
	}
	return hash, nil
}

// GetFileEntry retrieves the full file entry including scan time.
func (s *Store) GetFileEntry(path string) (*FileIndex, error) {
	var entry FileIndex
	var scannedAt string
	err := s.db.QueryRow(`
        SELECT file_path, scan_hash, scanned_at FROM file_index WHERE file_path = ?`,
		path).Scan(&entry.FilePath, &entry.ScanHash, &scannedAt)
	if err != nil {
		return nil, err
	}
	entry.ScannedAt, _ = time.Parse(time.RFC3339, scannedAt)
	return &entry, nil
}

// IsFileChanged checks if a file's content has changed since last scan.
// Returns true if the file has changed or has never been scanned.
func (s *Store) IsFileChanged(path, newHash string) (bool, error) {
	oldHash, err := s.GetFileHash(path)
	if err == sql.ErrNoRows {
		return true, nil // Never scanned
	}
	if err != nil {
		return false, err
	}
	return oldHash != newHash, nil
}

// GetAllFileEntries retrieves all file entries from the index.
func (s *Store) GetAllFileEntries() ([]*FileIndex, error) {
	rows, err := s.db.Query(`
        SELECT file_path, scan_hash, scanned_at FROM file_index ORDER BY file_path`)
	if err != nil {
		return nil, fmt.Errorf("query file entries: %w", err)
	}
	defer rows.Close()

	var entries []*FileIndex
	for rows.Next() {
		var entry FileIndex
		var scannedAt string
		err := rows.Scan(&entry.FilePath, &entry.ScanHash, &scannedAt)
		if err != nil {
			return nil, fmt.Errorf("scan row: %w", err)
		}
		entry.ScannedAt, _ = time.Parse(time.RFC3339, scannedAt)
		entries = append(entries, &entry)
	}
	return entries, rows.Err()
}

// DeleteFileEntry removes a file from the index.
func (s *Store) DeleteFileEntry(path string) error {
	_, err := s.db.Exec("DELETE FROM file_index WHERE file_path = ?", path)
	return err
}

// GetChangedFiles returns files that have changed compared to the provided hashes.
func (s *Store) GetChangedFiles(fileHashes map[string]string) ([]string, error) {
	var changed []string

	for path, newHash := range fileHashes {
		isChanged, err := s.IsFileChanged(path, newHash)
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
func (s *Store) PruneStaleEntries(validPaths map[string]bool) (int, error) {
	entries, err := s.GetAllFileEntries()
	if err != nil {
		return 0, err
	}

	var pruned int
	for _, entry := range entries {
		if !validPaths[entry.FilePath] {
			if err := s.DeleteFileEntry(entry.FilePath); err != nil {
				return pruned, err
			}
			pruned++
		}
	}

	return pruned, nil
}

// ClearFileIndex removes all file index data.
func (s *Store) ClearFileIndex() error {
	_, err := s.db.Exec("DELETE FROM file_index")
	return err
}

// CountFileIndex returns the number of indexed files.
func (s *Store) CountFileIndex() (int, error) {
	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM file_index").Scan(&count)
	return count, err
}
