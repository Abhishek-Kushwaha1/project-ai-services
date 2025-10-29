package image

import (
	"github.com/spf13/cobra"
)

var listImagesOpt bool

var ImageCmd = &cobra.Command{
	Use:   "image",
	Short: "Manage application images",
	Long:  ``,
	Args:  cobra.MaximumNArgs(0),
	RunE: func(cmd *cobra.Command, args []string) error {
		if listImagesOpt {
			cmd.Println("Listing application images available locally...")
		} else {
			cmd.Println("No administrative action specified. Use --help for more information.")
		}
		return nil
	},
}

func init() {
	ImageCmd.AddCommand(listCmd)
	// ImageCmd.Flags().BoolVarP(&listImagesOpt, "list-images", "l", false, "List application images available locally")
}
