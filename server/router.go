package server

import (
	"pkuphysu-backend/internal/config"
	"pkuphysu-backend/internal/db"
	"pkuphysu-backend/internal/logger"
	"pkuphysu-backend/server/handles"
	"pkuphysu-backend/server/middlewares"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	log "github.com/sirupsen/logrus"
)

func Init(e *gin.Engine) {
	config.InitConfig()
	logger.Init()
	e.Use(gin.LoggerWithWriter(log.StandardLogger().Out))
	e.Use(gin.RecoveryWithWriter(log.StandardLogger().Out))
	db.InitDB()

	e.POST("/user/register", handles.CreateUser)
	e.POST("/auth/login", handles.Login)
	e.POST("/iaaa/login", handles.IaaaLogin)
	e.POST("/email/send", handles.SendVerificationEmail)
	e.POST("/email/verify", handles.VerifyEmail)

	e.GET("/:file", handles.StaticFile)

	e.GET("/db-tables", handles.ListTables)

	g := e.Group("", middlewares.Auth())
	g.GET("/ping", func(c *gin.Context) {
		c.String(200, "pong")
	})
	g.GET("/user/me", handles.CurrentUser)
	g.DELETE("/user/me", handles.DeleteUser)
	g.PUT("/user/me", handles.UpdateUserInfo)
	g.POST("/user/avatar", handles.UploadAvatar)
	e.GET("/user/avatar/:id", handles.GetAvatar)
	g.POST("/auth/change-password", handles.ChangePassword)

	g.GET("/forum/posts", handles.GetPosts)
	g.GET("/forum/posts/:id", handles.GetPost)
	g.GET("/forum/comments/:id", handles.GetComments)
	g.POST("/forum/comments", handles.SubmitComment)
	g.POST("/forum/posts", handles.SubmitPost)
	g.GET("/forum/follow", handles.GetFollowedPosts)
	g.POST("/forum/follow/:id", handles.FollowPost)
	g.POST("/forum/like/:id", handles.LikePost)
	g.POST("/forum/comment/like/:id", handles.LikeComment)
	
	Cors(e)
}

func Cors(e *gin.Engine) {
	conf := cors.DefaultConfig()
	conf.AllowOrigins = config.Conf.Cors.AllowOrigins
	conf.AllowHeaders = config.Conf.Cors.AllowHeaders
	conf.AllowMethods = config.Conf.Cors.AllowMethods
	e.Use(cors.New(conf))
}