package sync

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/asqat/cloudir/internal/drive"
	"github.com/asqat/cloudir/internal/state"
	"github.com/asqat/cloudir/internal/watcher"
	"github.com/schollz/progressbar/v3"
	googledrive "google.golang.org/api/drive/v3"
)

type Engine struct {
	driveClient   *drive.Client
	store         *state.Store
	watcher       *watcher.Watcher
	localRoot     string
	driveFolderID string
	strategy      string
	dryRun        bool
	interval      time.Duration
}

func NewEngine(client *drive.Client, store *state.Store, w *watcher.Watcher, localRoot, driveFolderID, strategy string, dryRun bool, interval int) *Engine {
	return &Engine{
		driveClient:   client,
		store:         store,
		watcher:       w,
		localRoot:     localRoot,
		driveFolderID: driveFolderID,
		strategy:      strategy,
		dryRun:        dryRun,
		interval:      time.Duration(interval) * time.Second,
	}
}

func (e *Engine) Start(ctx context.Context) error {
	log.Println("Starting initial sync...")
	if err := e.InitialSync(ctx); err != nil {
		return fmt.Errorf("initial sync failed: %v", err)
	}
	log.Println("Initial sync completed.")

	log.Println("Starting real-time watch...")
	if err := e.watcher.Watch(e.localRoot); err != nil {
		return fmt.Errorf("failed to start watcher: %v", err)
	}

	ticker := time.NewTicker(e.interval)
	defer ticker.Stop()

	for {
		select {
		case event := <-e.watcher.Events:
			e.handleWatcherEvent(ctx, event)
		case <-ticker.C:
			log.Println("Checking for remote changes...")
			e.pullRemoteChanges(ctx)
		case <-ctx.Done():
			return nil
		}
	}
}

func (e *Engine) pullRemoteChanges(ctx context.Context) {
	remoteMap, err := e.scanRemote(ctx, e.driveFolderID, "")
	if err != nil {
		log.Printf("Error scanning remote: %v", err)
		return
	}

	for relPath, remoteFile := range remoteMap {
		meta, _ := e.store.GetFileByPath(relPath)
		fullPath := filepath.Join(e.localRoot, relPath)
		_, localExists := os.Stat(fullPath)

		if remoteFile.MimeType == "application/vnd.google-apps.folder" {
			if os.IsNotExist(localExists) {
				os.MkdirAll(fullPath, 0755)
			}
			continue
		}

		if meta == nil || remoteFile.Md5Checksum != meta.Hash {
			log.Printf("Remote change detected: %s", relPath)
			if os.IsNotExist(localExists) {
				e.downloadFile(ctx, remoteFile.Id, relPath)
			} else {
				e.resolveConflict(ctx, relPath, fullPath, remoteFile)
			}
		}
	}
}

func (e *Engine) InitialSync(ctx context.Context) error {
	// 1. Scan Local
	localFiles := make(map[string]os.FileInfo)
	err := filepath.Walk(e.localRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if path == e.localRoot {
			return nil
		}
		relPath, _ := filepath.Rel(e.localRoot, path)
		localFiles[relPath] = info
		return nil
	})
	if err != nil {
		return err
	}

	// 2. Scan Remote (recursively for initial sync)
	remoteMap, err := e.scanRemote(ctx, e.driveFolderID, "")
	if err != nil {
		return err
	}

	// 3. Local -> Remote Sync
	for relPath, info := range localFiles {
		fullPath := filepath.Join(e.localRoot, relPath)
		meta, _ := e.store.GetFileByPath(relPath)
		remoteFile, inRemote := remoteMap[relPath]

		if info.IsDir() {
			if !inRemote && meta == nil {
				e.createDirectory(ctx, relPath)
			}
			continue
		}

		if meta == nil {
			if inRemote {
				// Conflict: New local file, but also exists in remote
				e.resolveConflict(ctx, relPath, fullPath, remoteFile)
			} else {
				e.uploadFile(ctx, fullPath, relPath)
			}
		} else {
			// Existing local file
			if inRemote {
				if remoteFile.Md5Checksum != meta.Hash {
					e.resolveConflict(ctx, relPath, fullPath, remoteFile)
				}
			} else {
				// Deleted in remote, but exists locally?
				// For now, re-upload
				e.uploadFile(ctx, fullPath, relPath)
			}
		}
	}

	// 4. Remote -> Local Sync
	for relPath, remoteFile := range remoteMap {
		if _, inLocal := localFiles[relPath]; !inLocal {
			if remoteFile.MimeType == "application/vnd.google-apps.folder" {
				os.MkdirAll(filepath.Join(e.localRoot, relPath), 0755)
				e.store.SaveFile(state.FileMeta{
					LocalPath:   relPath,
					DriveID:     remoteFile.Id,
					IsDirectory: true,
				})
			} else {
				e.downloadFile(ctx, remoteFile.Id, relPath)
			}
		}
	}

	return nil
}

func (e *Engine) scanRemote(ctx context.Context, folderID, relPath string) (map[string]*googledrive.File, error) {
	result := make(map[string]*googledrive.File)
	files, err := e.driveClient.ListFiles(ctx, folderID)
	if err != nil {
		return nil, err
	}

	for _, f := range files {
		currentRel := filepath.Join(relPath, f.Name)
		result[currentRel] = f
		if f.MimeType == "application/vnd.google-apps.folder" {
			subFiles, err := e.scanRemote(ctx, f.Id, currentRel)
			if err != nil {
				return nil, err
			}
			for k, v := range subFiles {
				result[k] = v
			}
		}
	}
	return result, nil
}

func (e *Engine) resolveConflict(ctx context.Context, relPath, fullPath string, remoteFile *googledrive.File) {
	log.Printf("Conflict detected for %s. Strategy: %s", relPath, e.strategy)
	switch e.strategy {
	case "local":
		e.updateFile(ctx, fullPath, relPath, remoteFile.Id)
	case "remote":
		e.downloadFile(ctx, remoteFile.Id, relPath)
	default: // latest
		localInfo, _ := os.Stat(fullPath)
		remoteTime, _ := time.Parse(time.RFC3339, remoteFile.ModifiedTime)
		if localInfo.ModTime().After(remoteTime) {
			e.updateFile(ctx, fullPath, relPath, remoteFile.Id)
		} else {
			e.downloadFile(ctx, remoteFile.Id, relPath)
		}
	}
}

func (e *Engine) downloadFile(ctx context.Context, driveID, relPath string) {
	if e.dryRun {
		log.Printf("[DRY-RUN] Downloading %s", relPath)
		return
	}

	fullPath := filepath.Join(e.localRoot, relPath)
	os.MkdirAll(filepath.Dir(fullPath), 0755)

	body, err := e.driveClient.DownloadFile(ctx, driveID)
	if err != nil {
		log.Printf("Error downloading %s: %v", relPath, err)
		return
	}
	defer body.Close()

	// Get file size for progress bar if possible
	var size int64 = -1
	if f, err := e.driveClient.Service.Files.Get(driveID).Fields("size").Do(); err == nil {
		size = f.Size
	}

	bar := progressbar.DefaultBytes(size, "downloading "+relPath)

	out, err := os.Create(fullPath)
	if err != nil {
		log.Printf("Error creating file %s: %v", fullPath, err)
		return
	}
	defer out.Close()

	_, err = io.Copy(io.MultiWriter(out, bar), body)
	if err != nil {
		log.Printf("Error writing file %s: %v", fullPath, err)
		return
	}
	fmt.Println() // New line after progress bar

	hash, _ := calculateHash(fullPath)
	e.store.SaveFile(state.FileMeta{
		LocalPath:    relPath,
		DriveID:      driveID,
		Hash:         hash,
		ModifiedTime: time.Now().Unix(),
		IsDirectory:  false,
	})
}

func (e *Engine) createDirectory(ctx context.Context, relPath string) {
	if e.dryRun {
		log.Printf("[DRY-RUN] Creating directory %s", relPath)
		return
	}

	parentRel := filepath.Dir(relPath)
	parentID := e.driveFolderID
	if parentRel != "." {
		parentMeta, _ := e.store.GetFileByPath(parentRel)
		if parentMeta != nil {
			parentID = parentMeta.DriveID
		} else {
			// Parent doesn't exist yet, create it recursively
			e.createDirectory(ctx, parentRel)
			parentMeta, _ = e.store.GetFileByPath(parentRel)
			if parentMeta != nil {
				parentID = parentMeta.DriveID
			}
		}
	}

	driveFile, err := e.driveClient.CreateDirectory(ctx, filepath.Base(relPath), parentID)
	if err != nil {
		log.Printf("Error creating directory %s: %v", relPath, err)
		return
	}

	e.store.SaveFile(state.FileMeta{
		LocalPath:    relPath,
		DriveID:      driveFile.Id,
		IsDirectory:  true,
		ModifiedTime: time.Now().Unix(),
	})
}

func (e *Engine) handleWatcherEvent(ctx context.Context, event watcher.Event) {
	relPath, _ := filepath.Rel(e.localRoot, event.Path)
	log.Printf("Event: %v on %s", event.Op, relPath)

	switch event.Op {
	case watcher.Create, watcher.Write:
		info, err := os.Stat(event.Path)
		if err != nil {
			return
		}
		if info.IsDir() {
			e.createDirectory(ctx, relPath)
			return
		}

		meta, _ := e.store.GetFileByPath(relPath)
		if meta == nil {
			e.uploadFile(ctx, event.Path, relPath)
		} else {
			e.updateFile(ctx, event.Path, relPath, meta.DriveID)
		}
	case watcher.Remove:
		meta, _ := e.store.GetFileByPath(relPath)
		if meta != nil {
			e.deleteFile(ctx, relPath, meta.DriveID)
		}
	}
}

func (e *Engine) uploadFile(ctx context.Context, fullPath, relPath string) {
	if e.dryRun {
		log.Printf("[DRY-RUN] Uploading %s", relPath)
		return
	}

	parentRel := filepath.Dir(relPath)
	parentID := e.driveFolderID
	if parentRel != "." {
		parentMeta, _ := e.store.GetFileByPath(parentRel)
		if parentMeta != nil {
			parentID = parentMeta.DriveID
		}
	}

	f, err := os.Open(fullPath)
	if err != nil {
		log.Printf("Error opening file %s: %v", fullPath, err)
		return
	}
	defer f.Close()

	stat, _ := f.Stat()
	bar := progressbar.DefaultBytes(stat.Size(), "uploading "+relPath)

	driveFile, err := e.driveClient.UploadFile(ctx, filepath.Base(relPath), parentID, io.TeeReader(f, bar), "")
	if err != nil {
		log.Printf("Error uploading %s: %v", relPath, err)
		return
	}
	fmt.Println()

	hash, _ := calculateHash(fullPath)
	e.store.SaveFile(state.FileMeta{
		LocalPath:    relPath,
		DriveID:      driveFile.Id,
		Hash:         hash,
		ModifiedTime: time.Now().Unix(),
		IsDirectory:  false,
	})
}

func (e *Engine) updateFile(ctx context.Context, fullPath, relPath, driveID string) {
	if e.dryRun {
		log.Printf("[DRY-RUN] Updating %s", relPath)
		return
	}

	f, err := os.Open(fullPath)
	if err != nil {
		log.Printf("Error opening file %s: %v", fullPath, err)
		return
	}
	defer f.Close()

	_, err = e.driveClient.UpdateFile(ctx, driveID, f)
	if err != nil {
		log.Printf("Error updating %s: %v", relPath, err)
		return
	}

	hash, _ := calculateHash(fullPath)
	e.store.SaveFile(state.FileMeta{
		LocalPath:    relPath,
		DriveID:      driveID,
		Hash:         hash,
		ModifiedTime: time.Now().Unix(),
		IsDirectory:  false,
	})
}

func (e *Engine) deleteFile(ctx context.Context, relPath, driveID string) {
	if e.dryRun {
		log.Printf("[DRY-RUN] Deleting %s", relPath)
		return
	}

	err := e.driveClient.DeleteFile(ctx, driveID)
	if err != nil {
		log.Printf("Error deleting %s: %v", relPath, err)
		return
	}

	e.store.DeleteFile(relPath)
}

func calculateHash(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := md5.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}
