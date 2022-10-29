package commands

import (
	"fmt"
	"os"

	"github.com/nknorg/nkn/v2/api/httpjson/client"
	"github.com/spf13/cobra"
)

// modeCmd represents the syncMode command
var modeCmd = &cobra.Command{
	Use:   "mode",
	Short: "get sync mode of node",
	Long:  "Shows information about sync mode of node.",
	Run: func(cmd *cobra.Command, args []string) {
		modeAction()
	},
}

func init() {
	rootCmd.AddCommand(modeCmd)
}

func modeAction() (err error) {
	resp, err := client.Call(Address(), "getsyncmode", 0, nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return err
	}
	return FormatOutput(resp)
}
