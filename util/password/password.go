package password

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/util/config"

	"github.com/howeyc/gopass"
)

var (
	Passwd string
)

// GetPassword gets password from user input
func GetPassword() ([]byte, error) {
	fmt.Printf("Password:")
	passwd, err := gopass.GetPasswd()
	if err != nil {
		return nil, err
	}
	return passwd, nil
}

// GetConfirmedPassword gets double confirmed password from user input
func GetConfirmedPassword() ([]byte, error) {
	fmt.Printf("Password:")
	first, err := gopass.GetPasswd()
	if err != nil {
		return nil, err
	}

	if len(first) == 0 {
		fmt.Println("password is invalid.")
		os.Exit(1)
	}

	fmt.Printf("Re-enter Password:")
	second, err := gopass.GetPasswd()
	if err != nil {
		return nil, err
	}
	if len(first) != len(second) {
		fmt.Println("Unmatched Password")
		os.Exit(1)
	}
	for i, v := range first {
		if v != second[i] {
			fmt.Println("Unmatched Password")
			os.Exit(1)
		}
	}
	return first, nil
}

// GetAccountPassword gets the node's wallet password from the command line,
// the NKN_WALLET_PASSWORD environment variable, or user input, in this order
func GetAccountPassword() ([]byte, error) {
	if Passwd == "" && config.Parameters.PasswordFile != "" {
		passwordByte, err := ReadPasswordFile()
		if err != nil {
			fmt.Println(err)
		}
		pwd := string(passwordByte)
		Passwd = strings.Trim(pwd, "\r\n")
		if Passwd != "" {
			return []byte(Passwd), nil
		}
	}
	if Passwd == "" {
		Passwd = os.Getenv("NKN_WALLET_PASSWORD")
	}
	if !config.Parameters.WebGuiCreateWallet {
		if Passwd == "" {
			return GetPassword()
		}
	}
	if Passwd == "" {
		return []byte(Passwd), errors.New("wait for set password")
	}
	return []byte(Passwd), nil
}

func ReadPasswordFile() ([]byte, error) {
	var file []byte

	passwordFile := config.Parameters.PasswordFile

	_, err := os.Stat(passwordFile)
	if err != nil {
		return nil, err
	}

	file, err = ioutil.ReadFile(passwordFile)
	if err != nil {
		return nil, err
	}

	// Remove the UTF-8 Byte Order Mark
	file = bytes.TrimPrefix(file, []byte("\xef\xbb\xbf"))
	return file, nil
}

func SavePassword(pwd string) error {
	passwordFile := config.Parameters.PasswordFile

	if pwd != "" && passwordFile != "" && config.Parameters.WebGuiCreateWallet {
		if common.FileExisted(passwordFile) {
			return errors.New("wallet password file exists")
		}

		return ioutil.WriteFile(config.Parameters.PasswordFile, []byte(pwd), 0666)
	}

	return nil
}
