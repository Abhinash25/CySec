package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/cysec-env/cysec/internal/ui"
)

func newBannerCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "banner",
		Short: "Display the CySec.env banner",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println(ui.Banner())
			return nil
		},
	}
}
