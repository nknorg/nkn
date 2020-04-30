package dashboard

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/securecookie"
	"github.com/nknorg/nkn/dashboard/auth"
	serviceConfig "github.com/nknorg/nkn/dashboard/config"
	"github.com/nknorg/nkn/dashboard/routes"
	"github.com/nknorg/nkn/node"
	"github.com/nknorg/nkn/util/config"
	"github.com/nknorg/nkn/util/log"
	"github.com/nknorg/nkn/vault"
	"github.com/pborman/uuid"
)

var (
	localNode *node.LocalNode
	wallet    *vault.Wallet
	id        []byte
)

func Init(ln *node.LocalNode, w *vault.Wallet, d []byte) {
	if ln != nil {
		localNode = ln
		serviceConfig.IsNodeInit = true
	}
	if w != nil {
		wallet = w
		serviceConfig.IsWalletInit = true
	}
	if len(d) > 0 {
		id = d
		serviceConfig.IsIdInit = true
	}
}

func Start() {
	// build release settings
	gin.SetMode(gin.ReleaseMode)
	app := gin.New()
	app.Use(gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		log.WebLog.Infof("%s - [%s] \"%s %s %s %d %s\" %s \"%s\"",
			param.ClientIP,
			param.TimeStamp.Format(time.RFC1123),
			param.Method,
			param.Path,
			param.Request.Proto,
			param.StatusCode,
			param.Latency,
			param.Request.UserAgent(),
			param.ErrorMessage,
		)
		return ""
	}))

	app.Use(gin.Recovery())

	store := cookie.NewStore(securecookie.GenerateRandomKey(16), securecookie.GenerateRandomKey(16))
	store.Options(sessions.Options{
		MaxAge:   60, //30s
		Path:     "/",
		HttpOnly: false,
	})
	app.Use(sessions.Sessions("session", store))

	app.Use(func(context *gin.Context) {
		// init config
		if serviceConfig.IsNodeInit {
			context.Set("localNode", localNode)
		}
		if serviceConfig.IsWalletInit {
			context.Set("wallet", wallet)
		}
		if serviceConfig.IsIdInit {
			context.Set("id", id)
		}
	})

	app.Use(func(context *gin.Context) {
		// header
		context.Header("Access-Control-Allow-Origin", context.Request.Header.Get("Origin"))
		context.Header("Access-Control-Allow-Methods", "GET, POST, OPTIONS, PUT, PATCH, DELETE")
		context.Header("Access-Control-Allow-Headers", "Origin,X-Requested-With,content-type,Authorization,Unix,Accept,Token")
		context.Header("Access-Control-Allow-Credentials", "true")

		context.Next()
	})

	app.Use(func(context *gin.Context) {
		method := context.Request.Method
		// pass all OPTIONS method
		if method == "OPTIONS" {
			context.JSON(http.StatusOK, "Options")
		}
		context.Next()
	})

	app.HEAD("/api/verification", auth.WalletAuth(), func(context *gin.Context) {

	})

	app.Use(func(context *gin.Context) {
		session := sessions.Default(context)

		now := time.Now().Unix()
		if serviceConfig.TokenExp == 0 || serviceConfig.TokenExp+serviceConfig.TokenExpSec < now {
			token := uuid.NewUUID().String()
			serviceConfig.Token = token
			serviceConfig.TokenExp = now + serviceConfig.TokenExpSec

			session.Set("token", token)
			session.Save()
		}

		if session.Get("token") == nil {
			session.Set("token", serviceConfig.Token)
			session.Save()
		}

		context.Next()
	})

	app.StaticFS("/web", http.Dir("web"))

	// error route
	app.Use(func(context *gin.Context) {
		context.Next()

		err := context.Errors.Last()
		if err != nil && !context.Writer.Written() {
			context.JSON(http.StatusInternalServerError, err.Error())
		}
	})

	app.GET("/", func(context *gin.Context) {
		context.Redirect(301, "/web")
	})

	app.Use(routes.Routes(app))

	// 404 route
	app.Use(func(context *gin.Context) {
		context.JSON(http.StatusNotFound, "not found")
	})

	app.Run(config.Parameters.WebGuiListenAddress + ":" + strconv.Itoa(int(config.Parameters.WebGuiPort)))
}
