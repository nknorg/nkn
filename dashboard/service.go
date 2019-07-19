package dashboard

import (
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
	"net/http"
	"strconv"
	"time"
)

var (
	localNode *node.LocalNode
	wallet    vault.Wallet
)

func Init(ln *node.LocalNode, w vault.Wallet) {
	localNode = ln
	wallet = w
	serviceConfig.IsInit = true
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
		MaxAge:   30, //30s
		Path:     "/",
		HttpOnly: false,
	})
	app.Use(sessions.Sessions("session", store))

	app.Use(func(context *gin.Context) {
		// init config
		if serviceConfig.IsInit {
			context.Set("localNode", localNode)
			context.Set("wallet", wallet)
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
		token := session.Get("token")
		if token == nil {
			session.Set("token", uuid.NewUUID().String())
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

	app.Use(routes.Routes(app))

	// 404 route
	app.Use(func(context *gin.Context) {
		context.JSON(http.StatusNotFound, "not found")
	})

	app.Run(config.Parameters.WebGuiListenAddress + ":" + strconv.Itoa(int(config.Parameters.WebGuiPort)))
}
