package commands

import (
	"fmt"
	"os"

	"github.com/nknorg/nkn/v2/api/httpjson/client"
	"github.com/spf13/cobra"
)

// debugCmd represents the debug command
var debugCmd = &cobra.Command{
	Use:   "debug",
	Short: "blockchain node debugging",
	Long:  "With nknc debug, you could debug blockchain node.",
	Run: func(cmd *cobra.Command, args []string) {
		debugAction()
	},
}

var (
	level int
)

func init() {
	rootCmd.AddCommand(debugCmd)

	debugCmd.Flags().IntVarP(&level, "level", "l", -1, "log level 0-6")
	debugCmd.MarkFlagRequired("level")
}

func debugAction() (err error) {
	if level != -1 {
		resp, err := client.Call(Address(), "setdebuginfo", 0, map[string]interface{}{"level": level})
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return err
		}
		FormatOutput(resp)
	}
	return nil
}
