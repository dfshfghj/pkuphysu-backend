package main

import (
	"fmt"

	"pkuphysu-backend/internal/config"
	"pkuphysu-backend/server"

	"github.com/gin-gonic/gin"
)

func main() {
	gin.SetMode(gin.ReleaseMode)

	r := gin.New()

	server.Init(r)

	r.Run(fmt.Sprintf(":%d", config.Conf.Port))
}