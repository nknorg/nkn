package webservice

import (
	"github.com/gin-gonic/gin"
	"github.com/nknorg/nkn/util/config"
	"github.com/nknorg/nkn/util/log"
	"net/http"
	"strconv"
	"time"
)

func Start() {
	app := gin.Default()

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
	app.StaticFS("/web", http.Dir("dist"))
	//_ = flag.String("config", "./config/config.json","config file")
	//flag.Parse()

	//app.Use(routerv1.Sign())
	//	app.Use(routerv1.Router(app.Group("/api/v1")))
	//(&home.HomeRouter{}).RouterByGroup(app.Group("/api/v1"))
	app.GET("/ping", func(c *gin.Context) {
		log.WebLog.Infof("pong")
		log.Infof("pong")
		c.JSON(200, gin.H{
			"message": "pong",
		})
	})

	// 404 router
	app.Use(func(context *gin.Context) {
		context.JSON(http.StatusNotFound, "not found")
	})

	app.Run(":" + strconv.Itoa(int(config.Parameters.WebServicePort)))

}
