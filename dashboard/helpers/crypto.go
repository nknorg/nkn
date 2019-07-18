package helpers

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	serviceConfig "github.com/nknorg/nkn/dashboard/config"
	"github.com/nknorg/nkn/util/log"
	"net/http"
	"strconv"
	"time"
)

func BuildPwd(pwd string) []byte {
	key := []byte(pwd)
	h := hmac.New(sha256.New, key)
	h.Write(key)

	return h.Sum(nil)
}

func AesEncrypt(plaintext string, pwd string) (string, error) {
	key := BuildPwd(pwd)
	var iv = key[:aes.BlockSize]

	encrypted := make([]byte, len(plaintext))
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	encrypter := cipher.NewCFBEncrypter(block, iv)
	encrypter.XORKeyStream(encrypted, []byte(plaintext))
	return hex.EncodeToString(encrypted), nil
}

func AesDecrypt(encrypted string, pwd string) (string, error) {
	key := BuildPwd(pwd)
	var err error
	src, err := hex.DecodeString(encrypted)
	if err != nil {
		return "", err
	}
	var iv = key[:aes.BlockSize]
	decrypted := make([]byte, len(src))
	var block cipher.Block
	block, err = aes.NewCipher([]byte(key))
	if err != nil {
		return "", err
	}
	decrypter := cipher.NewCFBDecrypter(block, iv)
	decrypter.XORKeyStream(decrypted, src)
	return string(decrypted), nil
}

type BodyData struct {
	Data string `form:"data"`
}

func DecryptData(context *gin.Context) string {
	var body BodyData
	if err := context.ShouldBind(&body); err != nil {
		log.WebLog.Error(err)
		context.AbortWithError(http.StatusBadRequest, err)
		return ""
	}

	tick := time.Now().Unix()
	padding := int64(serviceConfig.UnixRange)
	session := sessions.Default(context)
	token := session.Get("token")
	if token == nil {
		context.AbortWithError(http.StatusForbidden, errors.New("403 Forbidden"))
		return ""
	}
	for i := tick - padding; i < tick+padding; i++ {
		jsonData, err := AesDecrypt(body.Data, token.(string)+strconv.FormatInt(i, 10))
		if err != nil {
			continue
		}
		var data map[string]interface{}
		err = json.Unmarshal([]byte(jsonData), &data)
		if err != nil {
			continue
		}
		return jsonData
	}

	context.AbortWithError(http.StatusBadRequest, errors.New("400 Bad request"))
	return ""
}

func EncryptData(context *gin.Context, sourceData interface{}) string {
	buf, err := json.Marshal(sourceData)
	if err != nil {
		return ""
	}
	tick := time.Now().Unix()
	session := sessions.Default(context)
	token := session.Get("token")
	data, err := AesEncrypt(string(buf), token.(string)+strconv.FormatInt(tick, 10))
	if err != nil {
		return ""
	}
	return data
}
