package commands

import (
	"fmt"

	"github.com/nknorg/nkn/v2/chain/store"
	"github.com/spf13/cobra"
)

// pruningCmd represents the pruning command
var pruningCmd = &cobra.Command{
	Use:   "pruning",
	Short: "state trie pruning for nknd",
	Long:  "state trie pruning for nknd",
	RunE: func(cmd *cobra.Command, args []string) error {
		return pruningAction(cmd)
	},
}

var (
	currentheight bool
	startheights  bool
	pruningB      bool
	seq           bool
	lowmem        bool
	dumpnodes     bool
	verifystate   bool
)

func init() {
	rootCmd.AddCommand(pruningCmd)

	pruningCmd.Flags().BoolVar(&currentheight, "currentheight", false, "current block height of offline db")
	pruningCmd.Flags().BoolVar(&startheights, "startheights", false, "start heights of refcount and pruning")
	pruningCmd.Flags().BoolVar(&pruningB, "pruning", false, "prune state trie")
	pruningCmd.Flags().BoolVar(&seq, "seq", false, "prune state trie sequential mode")
	pruningCmd.Flags().BoolVar(&lowmem, "lowmem", false, "prune state trie low memory mode")
	pruningCmd.Flags().BoolVar(&dumpnodes, "dumpnodes", false, "dump nodes of tri")
	pruningCmd.Flags().BoolVar(&verifystate, "verifystate", false, "verify state of ledger")
}

func pruningAction(cmd *cobra.Command) error {
	switch {
	case currentheight:
		cs, err := store.NewLedgerStore()
		if err != nil {
			return err
		}
		_, h, err := cs.GetCurrentBlockHashFromDB()
		if err != nil {
			return err
		}
		fmt.Println(h)
	case startheights:
		cs, err := store.NewLedgerStore()
		if err != nil {
			return err
		}
		refCountStartHeight, pruningStartHeight := cs.GetPruningStartHeight()
		fmt.Println(refCountStartHeight, pruningStartHeight)
	case pruningB:
		cs, err := store.NewLedgerStore()
		if err != nil {
			return err
		}

		if seq {
			err := cs.SequentialPrune()
			if err != nil {
				return err
			}
		} else if lowmem {
			err := cs.PruneStatesLowMemory(true)
			if err != nil {
				return err
			}
		} else {
			err := cs.PruneStates()
			if err != nil {
				return err
			}
		}
	case dumpnodes:
		cs, err := store.NewLedgerStore()
		if err != nil {
			return err
		}
		err = cs.TrieTraverse(true)
		if err != nil {
			return err
		}
	case verifystate:
		cs, err := store.NewLedgerStore()
		if err != nil {
			return err
		}
		err = cs.VerifyState()
		if err != nil {
			return err
		}
	default:
		return cmd.Usage()
	}

	return nil
}
