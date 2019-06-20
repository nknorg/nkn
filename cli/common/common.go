package common

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strconv"

	"github.com/nknorg/nkn/util/config"
	"github.com/nknorg/nkn/util/password"

	"github.com/urfave/cli"
)

var (
	Ip      string
	Port    string
	Version string
)

func NewIpFlag() cli.Flag {
	return cli.StringFlag{
		Name:        "ip",
		Usage:       "node's ip address",
		Value:       "localhost",
		Destination: &Ip,
	}
}

func NewPortFlag() cli.Flag {
	return cli.StringFlag{
		Name:        "port",
		Usage:       "node's RPC port",
		Value:       strconv.Itoa(int(config.Parameters.HttpJsonPort)),
		Destination: &Port,
	}
}

func Address() string {
	return "http://" + net.JoinHostPort(Ip, Port)
}

func PrintError(c *cli.Context, err error, cmd string) {
	fmt.Println("Incorrect Usage:", err)
	fmt.Println("")
	cli.ShowCommandHelp(c, cmd)
}

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

// WalletPassword prompts user to input wallet password when password is not
// specified from command line
func WalletPassword(passwd string) []byte {
	if passwd == "" {
		tmppasswd, _ := password.GetPassword()
		return tmppasswd
	} else {
		return []byte(passwd)
	}
}

func GetPassword(passwd string) []byte {
	var tmp []byte
	var err error
	if passwd != "" {
		tmp = []byte(passwd)
	} else {
		tmp, err = password.GetPassword()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
	return tmp
}
