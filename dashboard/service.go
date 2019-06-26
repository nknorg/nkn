package dashboard

import (
	"github.com/gin-gonic/gin"
	"github.com/nknorg/nkn/dashboard/routes"
	"github.com/nknorg/nkn/node"
	"github.com/nknorg/nkn/util/config"
	"github.com/nknorg/nkn/util/log"
	"github.com/nknorg/nkn/vault"
	"net/http"
	"strconv"
	"time"
)

var (
	localNode *node.LocalNode
	wallet    vault.Wallet
	isInit    = false
	app       = gin.Default()
)

func Init(ln *node.LocalNode, w vault.Wallet) {
	isInit = true
	localNode = ln
	wallet = w
}

func Start() {
	//gin.SetMode(gin.ReleaseMode)
	//app := gin.Default()
	app.Use(func(context *gin.Context) {
		if isInit {
			context.Set("localNode", localNode)
			context.Set("wallet", wallet)
		}
	})
	app.Use(gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		log.WebLog.Infof("%s - [%s] \"%s %s %s %d %s \"%s\" %s\"\n",
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
	//app.Static("/assets", "./assets")
	app.StaticFS("/web", http.Dir("dashboard/web/dist"))

	app.Use(routes.Routes(app))

	// 404 router
	app.Use(func(context *gin.Context) {
		context.JSON(http.StatusNotFound, "not found")
	})

	app.Run(":" + strconv.Itoa(int(config.Parameters.WebServicePort)))

}
