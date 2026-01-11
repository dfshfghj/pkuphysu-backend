package server

import (
	"pkuphysu-backend/internal/config"
	"pkuphysu-backend/internal/db"
	"pkuphysu-backend/server/handles"
	"pkuphysu-backend/server/middlewares"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)
func Init(e *gin.Engine) {
	config.InitConfig()
	db.InitDB()
	
	e.POST("/user/register", handles.CreateUser)
	e.POST("/auth/login", handles.Login)

	g := e.Group("/api", middlewares.Auth())
	g.GET("/ping",  func(c *gin.Context) {
		c.String(200, "pong")
	})
	Cors(e)
}

func Cors(e *gin.Engine) {
	conf := cors.DefaultConfig()
	conf.AllowOrigins = config.Conf.Cors.AllowOrigins
	conf.AllowHeaders = config.Conf.Cors.AllowHeaders
	conf.AllowMethods = config.Conf.Cors.AllowMethods
	e.Use(cors.New(conf))
}