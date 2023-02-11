package cmd

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/client"
	"github.com/gin-gonic/gin"
	"github.com/spf13/cobra"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type ServiceUpdateRequest struct {
	Token      string `json:"token" binding:"required"`
	Tag        string `json:"tag" binding:"required"`
	PullFirst  bool   `json:"pull_first"`
	StartFirst bool   `json:"start_first"`
	StopSignal string `json:"stop_signal"`
}

var AllowedStopSignals = [...]string{"QUIT", "SIGTERM", "SIGKILL"}
var DefaultStopSignal string = "SIGTERM"

var DockerHost string
var ServerHost string
var ServerPort string
var ServerDebug bool

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
				v1.POST("/service/update", serviceUpdate)
			}
		}
		router.NoRoute(func(ctx *gin.Context) { ctx.JSON(http.StatusNotFound, gin.H{}) })

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
	// dockerClient, err := client.NewClientWithOpts(client.WithHost(DockrHost)) //ToDo:fix this on new docker versions
	dockerClient, err := client.NewClientWithOpts(client.WithVersion("1.41"), client.WithHost(DockerHost))
	if err != nil {
		panic(err)
	}
	defer dockerClient.Close()

	nodeName, err := dockerClient.Info(context.Background())
	if err != nil {
		panic(err)
	}

	ctx.JSON(http.StatusOK, gin.H{"Node Name": nodeName.Name})
}

func serviceUpdate(ctx *gin.Context) {
	// dockerClient, err := client.NewClientWithOpts(client.WithHost(DockrHost)) //ToDo:fix this on new docker versions
	dockerClient, err := client.NewClientWithOpts(client.WithVersion("1.41"), client.WithHost(DockerHost))
	if err != nil {
		panic(err)
	}
	defer dockerClient.Close()

	bodyObject := ServiceUpdateRequest{}
	if err := ctx.BindJSON(&bodyObject); err != nil {
		ctx.AbortWithError(http.StatusUnprocessableEntity, err)
		return
	}

	if !isAllowedStopSignals(&bodyObject.StopSignal) {
		ctx.AbortWithError(http.StatusUnprocessableEntity, fmt.Errorf("stop signal not valid"))
		return
	}

	db, err := gorm.Open(sqlite.Open(DatabaseFile), &gorm.Config{})
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	var service Service
	queryResult := db.Where("token = ?", bodyObject.Token).Find(&service)
	if queryResult.RowsAffected != 1 {
		ctx.AbortWithError(http.StatusNotFound, err)
		return
	}

	// filterName := filters.NewArgs(filters.KeyValuePair{Key: "name", Value: service.ServiceName})
	// services, err := dockerClient.ServiceList(context.Background(), types.ServiceListOptions{Filters: filterName})
	targetService, _, err := dockerClient.ServiceInspectWithRaw(ctx, service.ServiceName, types.ServiceInspectOptions{})
	if err != nil {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{})
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

	if bodyObject.PullFirst {
		// authConfig := types.AuthConfig{} //ToDO:test this part with a private repo
		_, err = dockerClient.ImagePull(ctx, targetService.Spec.TaskTemplate.ContainerSpec.Image, types.ImagePullOptions{})
		if err != nil {
			ctx.AbortWithError(http.StatusUnprocessableEntity, fmt.Errorf("image:%v could not be pulled", targetService.Spec.TaskTemplate.ContainerSpec.Image))
			return
		}
	}

	_, err = dockerClient.ServiceUpdate(ctx, targetService.ID, targetService.Version, targetService.Spec, types.ServiceUpdateOptions{})
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, err)
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
