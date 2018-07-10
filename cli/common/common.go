package common

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"strconv"

	"github.com/nknorg/nkn/util/config"
	"github.com/nknorg/nkn/util/password"

	"github.com/urfave/cli"
)

const (
	DefaultConfigFile = "./config.json"
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
		Value:       strconv.Itoa(int(parsePort())),
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

func parsePort() uint16 {
	file, err := ioutil.ReadFile(DefaultConfigFile)
	if err != nil {
		log.Printf("Config file error: %v, use default parameters.", err)
		return 0
	}
	// Remove the UTF-8 Byte Order Mark
	file = bytes.TrimPrefix(file, []byte("\xef\xbb\xbf"))

	config := config.Configuration{}
	err = json.Unmarshal(file, &config)
	if err != nil {
		log.Printf("Unmarshal config file error: %v, use default parameters.", err)
		return 0
	}

	return config.HttpJsonPort
}
