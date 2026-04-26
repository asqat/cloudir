package cli

import (
	"context"
	"os"
	"path/filepath"

	"github.com/asqat/cloudir/internal/drive"
	"github.com/asqat/cloudir/internal/state"
	"github.com/asqat/cloudir/internal/sync"
	"github.com/asqat/cloudir/internal/watcher"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	localDir         string
	driveFolderID    string
	credentialsPath  string
	tokenPath        string
	dbPath           string
	interval         int
	dryRun           bool
	conflictStrategy string
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Start real-time synchronization",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		// 1. Setup Drive Client
		if credentialsPath == "" {
			credentialsPath = "credentials.json"
		}
		if tokenPath == "" {
			tokenPath = "token.json"
		}
		driveClient, err := drive.NewClient(ctx, credentialsPath, tokenPath)
		if err != nil {
			return err
		}

		// 2. Setup State Store
		if dbPath == "" {
			home, _ := os.UserHomeDir()
			dbPath = filepath.Join(home, ".cloudir", "state.db")
		}
		store, err := state.NewStore(dbPath)
		if err != nil {
			return err
		}
		defer store.Close()

		// 3. Setup Watcher
		ignored := []string{".DS_Store", "node_modules", ".git", ".cloudir"}
		w, err := watcher.NewWatcher(ignored)
		if err != nil {
			return err
		}
		defer w.Close()

		// 4. Start Engine
		engine := sync.NewEngine(driveClient, store, w, localDir, driveFolderID, conflictStrategy, dryRun, interval)
		return engine.Start(ctx)
	},
}

func init() {
	rootCmd.AddCommand(syncCmd)

	syncCmd.Flags().StringVarP(&localDir, "dir", "d", "", "local directory to sync (required)")
	syncCmd.Flags().StringVarP(&driveFolderID, "drive-folder-id", "i", "", "target Google Drive folder ID (required)")
	syncCmd.Flags().StringVar(&credentialsPath, "credentials", "", "path to Google API credentials JSON")
	syncCmd.Flags().StringVar(&tokenPath, "token", "token.json", "path to store/read OAuth token")
	syncCmd.Flags().StringVar(&dbPath, "db", "", "path to SQLite state database")
	syncCmd.Flags().IntVar(&interval, "interval", 30, "fallback polling interval in seconds")
	syncCmd.Flags().BoolVar(&dryRun, "dry-run", false, "simulate changes without applying")
	syncCmd.Flags().StringVar(&conflictStrategy, "conflict-strategy", "latest", "conflict resolution strategy: local | remote | latest")

	syncCmd.MarkFlagRequired("dir")
	syncCmd.MarkFlagRequired("drive-folder-id")

	viper.BindPFlag("dir", syncCmd.Flags().Lookup("dir"))
	viper.BindPFlag("drive_folder_id", syncCmd.Flags().Lookup("drive-folder-id"))
}
