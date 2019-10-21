package service

import (
	"encoding/json"
	"fmt"
	. "github.com/nknorg/nkn/cli/common"
	"github.com/nknorg/nkn/util/log"
	"github.com/urfave/cli"
	"io/ioutil"
	"net/http"
	"time"
)

const requestTimeout = 5 * time.Second

func serviceAction(c *cli.Context) error {
	if c.NumFlags() == 0 {
		cli.ShowSubcommandHelp(c)
		return nil
	}

	switch {
	case c.Bool("list"):
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

		fmt.Println(out["post_stream"].(map[string]interface{})["posts"].([]interface{})[0].(map[string]interface{})["raw"])
	default:
		cli.ShowSubcommandHelp(c)
		return nil
	}

	return nil
}

func NewCommand() *cli.Command {
	return &cli.Command{
		Name:        "service",
		Usage:       "NKN-based service",
		Description: "NKN-based service.",
		ArgsUsage:   "[args]",
		Flags: []cli.Flag{
			cli.BoolFlag{
				Name:  "list, ls",
				Usage: "show nkn service list",
			},
		},
		Action: serviceAction,
		OnUsageError: func(c *cli.Context, err error, isSubcommand bool) error {
			PrintError(c, err, "service")
			return cli.NewExitError("", 1)
		},
	}
}
