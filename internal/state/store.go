package state

import (
	"database/sql"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

type FileMeta struct {
	LocalPath    string
	DriveID      string
	Hash         string
	ModifiedTime int64
	IsDirectory  bool
}

type Store struct {
	db *sql.DB
}

func NewStore(dbPath string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		return nil, err
	}

	s := &Store{db: db}
	if err := s.initSchema(); err != nil {
		return nil, err
	}

	return s, nil
}

func (s *Store) initSchema() error {
	query := `
	CREATE TABLE IF NOT EXISTS files (
		local_path TEXT PRIMARY KEY,
		drive_id TEXT,
		hash TEXT,
		modified_time INTEGER,
		is_directory BOOLEAN
	);`
	_, err := s.db.Exec(query)
	return err
}

func (s *Store) SaveFile(meta FileMeta) error {
	query := `INSERT OR REPLACE INTO files (local_path, drive_id, hash, modified_time, is_directory) VALUES (?, ?, ?, ?, ?)`
	_, err := s.db.Exec(query, meta.LocalPath, meta.DriveID, meta.Hash, meta.ModifiedTime, meta.IsDirectory)
	return err
}

func (s *Store) GetFileByPath(localPath string) (*FileMeta, error) {
	query := `SELECT local_path, drive_id, hash, modified_time, is_directory FROM files WHERE local_path = ?`
	row := s.db.QueryRow(query, localPath)
	var meta FileMeta
	err := row.Scan(&meta.LocalPath, &meta.DriveID, &meta.Hash, &meta.ModifiedTime, &meta.IsDirectory)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &meta, nil
}

func (s *Store) DeleteFile(localPath string) error {
	query := `DELETE FROM files WHERE local_path = ?`
	_, err := s.db.Exec(query, localPath)
	return err
}

func (s *Store) ListFiles() ([]FileMeta, error) {
	query := `SELECT local_path, drive_id, hash, modified_time, is_directory FROM files`
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var files []FileMeta
	for rows.Next() {
		var meta FileMeta
		if err := rows.Scan(&meta.LocalPath, &meta.DriveID, &meta.Hash, &meta.ModifiedTime, &meta.IsDirectory); err != nil {
			return nil, err
		}
		files = append(files, meta)
	}
	return files, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}
