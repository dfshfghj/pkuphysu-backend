package server

import (
	"pkuphysu-backend/internal/config"
	"pkuphysu-backend/internal/db"
	"pkuphysu-backend/internal/logger"
	"pkuphysu-backend/internal/moderation"
	"pkuphysu-backend/internal/wechat"
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
	moderation.Init()
	wechat.Init()

	e.Use(middlewares.RateLimit())

	e.POST("/auth/login", handles.Login)
	e.POST("/iaaa/login", handles.IaaaLogin)
	e.POST("/email/send", handles.SendVerificationEmail)
	e.POST("/email/verify", handles.VerifyEmail)

	e.GET("/static/*file", handles.StaticFile)
	e.POST("/files/upload", handles.UploadFile)
	e.GET("/files/*filename", handles.StaticFile)

	dba := e.Group("/dba", middlewares.Auth(), middlewares.AuthAdmin())
	dba.POST("/db-tables/create-all", handles.CreateAll)
	dba.GET("/db-tables", handles.ListTables)
	dba.GET("/db-tables/:table", handles.GetTableData)
	dba.DELETE("/db-tables/:table", handles.DeleteTableRecords)
	dba.PUT("/db-tables/:table", handles.UpsertTableRecords)
	dba.PATCH("/db-tables/:table", handles.UpsertTableRecords)
	dba.GET("/db-tables/migrate", handles.CheckMigration)
	dba.POST("/db-tables/migrate", handles.ExecuteMigration)

	e.GET("/ping", func(c *gin.Context) {
		c.String(200, "pong")
	})

	g := e.Group("", middlewares.Auth())

	g.GET("/user/me", handles.CurrentUser)
	g.DELETE("/user/me", handles.DeleteUser)
	g.PUT("/user/me", handles.UpdateUserInfo)
	g.POST("/user/avatar", handles.UploadAvatar)
	e.GET("/user/avatar/:id", handles.GetAvatar)
	g.POST("/auth/change-password", handles.ChangePassword)
	g.GET("/users", handles.ListUsers)
	g.GET("/users/search", handles.SearchUsers)
	g.GET("/admins", handles.ListAdmins)

	g.GET("/users/:id/profile", handles.GetUserProfile)
	g.GET("/users/:id/stats", handles.GetUserStats)
	g.GET("/user/me/profile", handles.GetMyProfileRaw)
	g.PUT("/user/me/profile", handles.UpdateMyProfile)

	g.GET("/notifications", handles.GetNotifications)

	g.GET("/forum/posts", handles.GetPosts)
	g.GET("/forum/posts/:id", handles.GetPost)
	g.PUT("/forum/posts/:id", handles.UpdatePost)
	g.DELETE("/forum/posts/:id", handles.DeleteOwnPost)
	g.GET("/forum/posts/:id/versions", handles.GetPostVersions)
	g.GET("/forum/comments/:id", handles.GetComments)
	g.DELETE("/forum/comments/:id", handles.DeleteOwnComment)
	g.POST("/forum/comments", handles.SubmitComment)
	g.POST("/forum/posts", handles.SubmitPost)
	g.POST("/forum/posts/:id/report", handles.ReportPost)
	g.POST("/forum/comments/:id/report", handles.ReportComment)
	g.GET("/forum/follow", handles.GetFollowedPosts)
	g.POST("/forum/follow/:id", handles.FollowPost)
	g.POST("/forum/like/:id", handles.LikePost)
	g.POST("/forum/comment/like/:id", handles.LikeComment)
	g.GET("/forum/tags", handles.GetTags)
	g.GET("/forum/post-quotes", handles.GetPostQuotes)
	e.POST("/markdown/preview", handles.Markdown)
	g.GET("/forum/posts/raw/:id", handles.GetRawPost)
	g.GET("/forum/comments/raw/:id", handles.GetRawComment)

	admin := e.Group("/admin", middlewares.Auth(), middlewares.AuthAdmin())
	admin.DELETE("/forum/posts/:id", handles.DeletePostByID)
	admin.DELETE("/forum/comments/:id", handles.DeleteCommentByID)
	admin.POST("/forum/posts/:id/review", handles.ReviewPostByID)
	admin.POST("/forum/comments/:id/review", handles.ReviewCommentByID)
	admin.GET("/forum/reports", handles.ListForumReports)
	admin.POST("/user/create", handles.CreateUser)

	we := e.Group("/wechat")
	we.Any("", handles.Message)

	Cors(e)
}

func Cors(e *gin.Engine) {
	conf := cors.DefaultConfig()
	conf.AllowOrigins = config.Conf.Cors.AllowOrigins
	conf.AllowHeaders = config.Conf.Cors.AllowHeaders
	conf.AllowMethods = config.Conf.Cors.AllowMethods
	e.Use(cors.New(conf))
}
