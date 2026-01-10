package server

import (
	"pkuphysu-backend/internal/config"
	"pkuphysu-backend/internal/db"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)
func Init(e *gin.Engine) {
	db.InitDB()
	Cors(e)

}

func Cors(e *gin.Engine) {
	conf := cors.DefaultConfig()
	conf.AllowOrigins = config.Conf.Cors.AllowOrigins
	conf.AllowHeaders = config.Conf.Cors.AllowHeaders
	conf.AllowMethods = config.Conf.Cors.AllowMethods
	e.Use(cors.New(conf))
}