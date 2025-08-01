package cmd

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/client"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/spf13/cobra"
	"gorm.io/gorm"
)

type ServiceStatusRequest struct {
	Token string `json:"token" binding:"required"`
}

type ServiceUpdateRequest struct {
	Token      string `json:"token" binding:"required"`
	Tag        string `json:"tag" binding:"required"`
	StartFirst bool   `json:"start_first"`
	StopSignal string `json:"stop_signal"`
}

var (
	AllowedStopSignals        = [...]string{"QUIT", "SIGTERM", "SIGKILL"}
	DefaultStopSignal  string = "SIGTERM"

	DockerHost  string
	ServerHost  string
	ServerPort  string
	ServerDebug bool
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "start an api web server for updating services",
	Run: func(cmd *cobra.Command, args []string) {
		migrate()
		ginMode := gin.ReleaseMode
		if ServerDebug {
			ginMode = gin.DebugMode
		}
		gin.SetMode(ginMode)
		router := gin.Default()

		api := router.Group("/api")
		{
			v1 := api.Group("/v1")
			{
				v1.GET("/node", node)
				v1.POST("/service/status", serviceStatus)
				v1.POST("/service/update", serviceUpdate)
			}
		}
		router.NoRoute(func(ctx *gin.Context) { ctx.JSON(http.StatusNotFound, gin.H{"error":"Route not found"}) })

		fmt.Println("version: " + version)
		fmt.Println("trying to bind to " + ServerHost + ":" + ServerPort)
		router.Run(ServerHost + ":" + ServerPort)
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)
	serveCmd.Flags().StringVarP(&DockerHost, "docker", "S", "unix:///var/run/docker.sock", "docker host")
	serveCmd.Flags().StringVarP(&ServerHost, "host", "H", "127.0.0.1", "listen host")
	serveCmd.Flags().StringVarP(&ServerPort, "port", "P", "8314", "listen port")
	serveCmd.Flags().BoolVar(&ServerDebug, "debug", false, "debug mode gin")
}

func node(ctx *gin.Context) {
	dockerClient, err := client.NewClientWithOpts(client.WithHost(DockerHost))
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer dockerClient.Close()

	nodeName, err := dockerClient.Info(context.Background())
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"node_name": nodeName.Name})
}

func serviceStatus(ctx *gin.Context) {
	dockerClient, err := client.NewClientWithOpts(client.WithHost(DockerHost))
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer dockerClient.Close()

	bodyObject := ServiceStatusRequest{}
	if err := ctx.BindJSON(&bodyObject); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	db, err := gorm.Open(sqlite.Open(DatabaseFile), &gorm.Config{})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	var service Service
	queryResult := db.Where("token = ?", bodyObject.Token).Find(&service)
	if queryResult.RowsAffected != 1 {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "token not found"})
		return
	}

	targetService, _, err := dockerClient.ServiceInspectWithRaw(ctx, service.ServiceName, swarm.ServiceInspectOptions{})
	if err != nil {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "docker service could not be retrieved or non existent"})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"service": service.ServiceName,
		"image":   targetService.Spec.TaskTemplate.ContainerSpec.Image,
	})
}

func serviceUpdate(ctx *gin.Context) {
	dockerClient, err := client.NewClientWithOpts(client.WithHost(DockerHost))
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer dockerClient.Close()

	bodyObject := ServiceUpdateRequest{}
	err = ctx.ShouldBindJSON(&bodyObject)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if !isAllowedStopSignals(&bodyObject.StopSignal) {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "stop signal not valid"})
		return
	}

	db, err := gorm.Open(sqlite.Open(DatabaseFile), &gorm.Config{})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	var service Service
	queryResult := db.Where("token = ?", bodyObject.Token).Find(&service)
	if queryResult.RowsAffected != 1 {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "token not found"})
		return
	}

	targetService, _, err := dockerClient.ServiceInspectWithRaw(ctx, service.ServiceName, swarm.ServiceInspectOptions{})
	if err != nil {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "docker service could not be retrieved or non existent"})
		return
	}

	oldRepository, _ := repoAndTagFromImage(targetService.Spec.TaskTemplate.ContainerSpec.Image)
	targetService.Spec.TaskTemplate.ContainerSpec.Image = oldRepository + bodyObject.Tag
	targetService.Spec.TaskTemplate.ContainerSpec.StopSignal = bodyObject.StopSignal
	targetService.Spec.UpdateConfig = &swarm.UpdateConfig{
		FailureAction: swarm.UpdateFailureActionRollback,
	}
	if bodyObject.StartFirst {
		targetService.Spec.UpdateConfig = &swarm.UpdateConfig{
			FailureAction: swarm.UpdateFailureActionRollback,
			Order:         swarm.UpdateOrderStartFirst,
		}
	}

	targetService.Spec.TaskTemplate.ContainerSpec.Env = append(targetService.Spec.TaskTemplate.ContainerSpec.Env, "FLOCKMAN_IMAGE_TAG=" + bodyObject.Tag)
	targetService.Spec.TaskTemplate.ContainerSpec.Env = append(targetService.Spec.TaskTemplate.ContainerSpec.Env, "FLOCKMAN_IMAGE_REPO=" + oldRepository)

	_, err = dockerClient.ServiceUpdate(ctx, targetService.ID, targetService.Version, targetService.Spec, swarm.ServiceUpdateOptions{})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"service": service.ServiceName,
		"image":   targetService.Spec.TaskTemplate.ContainerSpec.Image,
	})
}

func isAllowedStopSignals(needle *string) (result bool) {
	if *needle == "" {
		*needle = DefaultStopSignal
		return true
	}

	for _, v := range AllowedStopSignals {
		if v == *needle {
			return true
		}
	}

	return false
}

func repoAndTagFromImage(image string) (repo string, tag string) {
	repo = strings.SplitAfter(strings.SplitAfter(image, "@")[0], ":")[0]
	tag = strings.TrimSuffix(strings.SplitAfter(strings.SplitAfter(image, "@")[0], ":")[1], "@")
	return
}
