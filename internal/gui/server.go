package gui

import (
	"net/http"

	"github.com/amsen20/ecmus/internal/model"
	"github.com/amsen20/ecmus/internal/scheduler"
	"github.com/gin-gonic/gin"
)

var clusterStateRequestStream chan<- struct{}
var clusterStateStream <-chan *model.ClusterState
var engine *gin.Engine

func SetUp(bridge scheduler.SchedulerBridge) {
	clusterStateStream = bridge.ClusterStateStream
	clusterStateRequestStream = bridge.ClusterStateRequestStream

	engine = gin.Default()
	engine.LoadHTMLFiles("./internal/gui/index.html")
	engine.POST("/state", func(ctx *gin.Context) {
		clusterStateRequestStream <- struct{}{}
		ctx.JSON(http.StatusOK, gin.H{
			"content": (<-clusterStateStream).Display(),
		})
	})
	engine.GET("/", func(ctx *gin.Context) {
		ctx.HTML(http.StatusOK, "index.html", gin.H{})
	})
}

func Run() {
	engine.Run()
}
