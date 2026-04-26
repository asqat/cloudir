package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/asqat/cloudir/internal/state"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show synchronization status",
	RunE: func(cmd *cobra.Command, args []string) error {
		home, _ := os.UserHomeDir()
		dbPath := filepath.Join(home, ".cloudir", "state.db")
		store, err := state.NewStore(dbPath)
		if err != nil {
			return err
		}
		defer store.Close()

		files, err := store.ListFiles()
		if err != nil {
			return err
		}

		fmt.Printf("Sync Status: %d files tracked\n", len(files))
		for _, f := range files {
			typeStr := "FILE"
			if f.IsDirectory {
				typeStr = "DIR "
			}
			fmt.Printf("[%s] %s (ID: %s)\n", typeStr, f.LocalPath, f.DriveID)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
