package commands

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/nknorg/nkn/v2/util/log"
	"github.com/spf13/cobra"
)

const requestTimeout = 5 * time.Second

// serviceCmd represents the service command
var serviceCmd = &cobra.Command{
	Use:   "service",
	Short: "NKN-based service",
	Long:  "NKN-based service.",
	RunE: func(cmd *cobra.Command, args []string) error {
		return serviceAction(cmd)
	},
}

var (
	list bool
)

func init() {
	rootCmd.AddCommand(serviceCmd)

	serviceCmd.Flags().BoolVarP(&list, "list", "l", false, "show nkn service list")
}

func serviceAction(cmd *cobra.Command) error {
	switch {
	case list:
		var netClient = &http.Client{
			Timeout: requestTimeout,
		}
		resp, err := netClient.Get("https://forum.nkn.org/t/1836.json?include_raw=1")
		if err != nil {
			log.Errorf("GET request: %v\n", err)
			return err
		}
		defer resp.Body.Close()

		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Errorf("GET response: %v\n", err)
			return err
		}
		var out = make(map[string]interface{})
		err = json.Unmarshal(body, &out)
		if err != nil {
			log.Errorf("Unmarshal response: %v\n", err)
			return err
		}

		fmt.Println(out["post_stream"].(map[string]interface{})["posts"].([]interface{})[0].(map[string]interface{})["raw"])
	default:
		return cmd.Usage()
	}

	return nil
}
