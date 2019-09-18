package pruning

import (
	"errors"
	"fmt"

	"github.com/nknorg/nkn/chain/store"
	"github.com/urfave/cli"
)

func pruningAction(c *cli.Context) error {
	if c.NumFlags() == 0 {
		cli.ShowSubcommandHelp(c)
		return nil
	}

	switch {
	case c.Bool("currentheight"):
		cs, err := store.NewLedgerStore()
		if err != nil {
			fmt.Println(err)
			return err
		}
		_, h, err := cs.GetCurrentBlockHashFromDB()
		if err != nil {
			fmt.Println(err)
			return err
		}
		fmt.Println(h)
	case c.Bool("startheight"):
		cs, err := store.NewLedgerStore()
		if err != nil {
			return err
		}
		refCountStartHeight, pruningStartHeight := cs.GetPruningStartHeight()
		fmt.Println(refCountStartHeight, pruningStartHeight)
	case c.Bool("pruning"):
		cs, err := store.NewLedgerStore()
		if err != nil {
			return err
		}

		refCountStartHeight, pruningStartHeight := cs.GetPruningStartHeight()
		if refCountStartHeight == 0 && pruningStartHeight == 0 {
			err := cs.SequentialPrune()
			if err != nil {
				return err
			}
		} else if refCountStartHeight > 0 && pruningStartHeight > 0 {
			err := cs.PruneStates()
			if err != nil {
				return err
			}
		} else {
			return errors.New("get start height of pruning error")
		}
	case c.Bool("traverse"):
		cs, err := store.NewLedgerStore()
		if err != nil {
			return err
		}
		err = cs.TrieTraverse()
		if err != nil {
			return err
		}
	default:
		cli.ShowSubcommandHelp(c)
		return nil
	}

	return nil
}

func NewCommand() *cli.Command {
	return &cli.Command{
		Name:        "pruning",
		Usage:       "state trie pruning for nknd",
		Description: "state trie pruning for nknd.",
		ArgsUsage:   "[args]",
		Flags: []cli.Flag{
			cli.BoolFlag{
				Name:  "currentheight",
				Usage: "current block height of offline db",
			},
			cli.BoolFlag{
				Name:  "pruning",
				Usage: "prune state trie",
			},
			cli.BoolFlag{
				Name:  "startheight",
				Usage: "start height",
			},
			cli.BoolFlag{
				Name:  "traverse",
				Usage: "traverse trie",
			},
		},
		Action: pruningAction,
		OnUsageError: func(c *cli.Context, err error, isSubcommand bool) error {
			return cli.NewExitError("", 1)
		},
	}
}
