package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/asqat/cloudir/internal/drive"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Setup Google Drive credentials",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Starting cloudir initialization...")

		fmt.Print("Path to credentials.json: ")
		var credPath string
		fmt.Scanln(&credPath)
		if credPath == "" {
			credPath = "credentials.json"
		}

		if _, err := os.Stat(credPath); os.IsNotExist(err) {
			return fmt.Errorf("credentials file not found at %s. Please download it from Google Cloud Console", credPath)
		}

		ctx := context.Background()
		_, err := drive.NewClient(ctx, credPath, "token.json")
		if err != nil {
			return err
		}

		fmt.Println("\nSuccessfully authenticated with Google Drive!")
		fmt.Println("You can now use 'cloudir sync' to synchronize your directories.")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}
