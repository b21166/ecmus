// A very simple gin HTTP server
// for getting scheduler point of view
// from the cluster using a web page
// The gui sends an empty struct to scheduler bridge
// and the scheduler sends back a clone of state
// to gui which displays it using a simple HTML file.
package gui

import (
	"net/http"

	"github.com/amsen20/ecmus/internal/model"
	"github.com/amsen20/ecmus/internal/scheduler"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

var clusterStateRequestStream chan<- struct{}
var clusterStateStream <-chan *model.ClusterState
var router *gin.Engine

func registerRoutes() {
	router.POST("/state", func(ctx *gin.Context) {
		clusterStateRequestStream <- struct{}{}
		ctx.JSON(http.StatusOK, gin.H{
			"content": (<-clusterStateStream).Display(),
		})
	})

	router.GET("/", func(ctx *gin.Context) {
		ctx.HTML(http.StatusOK, "index.html", gin.H{})
	})
}

func SetUp(bridge scheduler.SchedulerBridge) {
	clusterStateStream = bridge.ClusterStateStream
	clusterStateRequestStream = bridge.ClusterStateRequestStream

	router = gin.Default()
	router.LoadHTMLFiles("./internal/gui/index.html")

	router.Use(cors.Default())

	registerRoutes()
}

func Run() {
	router.Run(":8080")
}
