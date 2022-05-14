package commands

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand"
	"net"
	"os"
	"strconv"
	"time"

	"github.com/howeyc/gopass"
	"github.com/nknorg/nkn/v2/config"
	"github.com/spf13/cobra"
)

// Globals
var (
	walletFile     string
	walletPassword string
	nonce          uint64
	fee            string
	ip             string
	port           string
)

var rootCmd = &cobra.Command{
	Use:     "nknc",
	Version: config.Version,
	Short:   "nknc - A cli tool for the NKN blockchain",
	Long:    "",
}

// RootCmd function  
func RootCmd() *cobra.Command {
	return rootCmd
}

// Execute function  
func Execute() {
	cobra.CheckErr(rootCmd.Execute())
}

// init function  
func init() {
	rand.Seed(time.Now().UnixNano())

	rootCmd.CompletionOptions.DisableDefaultCmd = true
	rootCmd.PersistentFlags().StringVar(&ip, "ip", "localhost", "node's ip address")
	rootCmd.PersistentFlags().StringVar(&port, "port", strconv.Itoa(int(config.Parameters.HttpJsonPort)), "node's ip address")
}

// Address function  
func Address() string {
	return "http://" + net.JoinHostPort(ip, port)
}

// FormatOutput function  
func FormatOutput(o []byte) error {
	var out bytes.Buffer
	err := json.Indent(&out, o, "", "\t")
	if err != nil {
		return err
	}
	out.Write([]byte("\n"))
	_, err = out.WriteTo(os.Stdout)

	return err
}

// GetPassword function  
func GetPassword(passwd string) []byte {
	var tmp []byte
	var err error

	if passwd != "" {
		tmp = []byte(passwd)
	} else {
		fmt.Printf("%s:", "Password")
		tmp, err = gopass.GetPasswd()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
	return tmp
}
