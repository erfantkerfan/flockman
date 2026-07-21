package cmd

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"slices"
	"strings"
	"syscall"
	"time"

	nanoid "github.com/aidarkhanov/nanoid/v2"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/client"
	"github.com/gin-gonic/gin"
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
	MetricsHost string
	MetricsPort string
	ServerDebug bool
)

func newDockerClient() (*client.Client, error) {
	return client.NewClientWithOpts(client.WithHost(DockerHost))
}

func findServiceByToken(token string) (*Service, error) {
	var service Service
	if err := db.Where("token = ?", token).First(&service).Error; err != nil {
		return nil, err
	}
	return &service, nil
}

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "start an api web server for updating services",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := validateMetricsPort(MetricsHost, MetricsPort); err != nil {
			return err
		}

		if err := initDB(); err != nil {
			return err
		}

		initMetrics()

		ginMode := gin.ReleaseMode
		if ServerDebug {
			ginMode = gin.DebugMode
		}
		gin.SetMode(ginMode)
		router := gin.Default()
		router.Use(metricsMiddleware())

		api := router.Group("/api")
		{
			v1 := api.Group("/v1")
			{
				v1.GET("/health", health)
				v1.GET("/node", node)
				v1.POST("/service/status", serviceStatus)
				v1.POST("/service/update", serviceUpdate)
			}
		}
		router.NoRoute(func(ctx *gin.Context) { ctx.JSON(http.StatusNotFound, gin.H{"error": "Route not found"}) })

		fmt.Println("version: " + version)
		fmt.Println("trying to bind to " + ServerHost + ":" + ServerPort)

		srv := &http.Server{
			Addr:    ServerHost + ":" + ServerPort,
			Handler: router,
		}

		go func() {
			if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				fmt.Fprintf(os.Stderr, "listen error: %s\n", err)
			}
		}()

		var metricsSrv *http.Server
		if MetricsPort != "" {
			metricsAddr := MetricsHost + ":" + MetricsPort
			metricsSrv = newMetricsServer(metricsAddr)
			fmt.Println("metrics exporter listening on " + metricsAddr)
			go func() {
				if err := metricsSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
					fmt.Fprintf(os.Stderr, "metrics listen error: %s\n", err)
				}
			}()
		}

		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		<-quit
		fmt.Println("\nshutting down server...")

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			return fmt.Errorf("server forced to shutdown: %w", err)
		}
		if metricsSrv != nil {
			mctx, mcancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer mcancel()
			if err := metricsSrv.Shutdown(mctx); err != nil {
				return fmt.Errorf("metrics server forced to shutdown: %w", err)
			}
		}

		fmt.Println("server exited gracefully")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)
	serveCmd.Flags().StringVarP(&DockerHost, "docker", "S", "unix:///var/run/docker.sock", "docker host")
	serveCmd.Flags().StringVarP(&ServerHost, "host", "H", "127.0.0.1", "listen host")
	serveCmd.Flags().StringVarP(&ServerPort, "port", "P", "8314", "listen port")
	serveCmd.Flags().StringVarP(&MetricsPort, "metrics-port", "M", "", "prometheus metrics listen port (disabled when empty)")
	serveCmd.Flags().StringVarP(&MetricsHost, "metrics-host", "", "127.0.0.1", "prometheus metrics listen host")
	serveCmd.Flags().BoolVar(&ServerDebug, "debug", false, "debug mode gin")
}

func health(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func node(ctx *gin.Context) {
	dc, err := newDockerClient()
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer dc.Close()

	started := time.Now()
	info, err := dc.Info(ctx)
	observeDockerAPI("info", started, err)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"node_name": info.Name, "flockman_version": version})
}

func serviceStatus(ctx *gin.Context) {
	var body ServiceStatusRequest
	if err := ctx.BindJSON(&body); err != nil {
		recordServiceStatus("bad_request")
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if !isValidTokenFormat(body.Token) {
		recordTokenLookup("invalid_format")
		recordServiceStatus("bad_request")
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid token format"})
		return
	}

	service, err := findServiceByToken(body.Token)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			recordTokenLookup("not_found")
			recordServiceStatus("unauthorized")
			ctx.JSON(http.StatusNotFound, gin.H{"error": "token not found"})
			return
		}
		recordTokenLookup("error")
		recordServiceStatus("error")
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	recordTokenLookup("found")

	dc, err := newDockerClient()
	if err != nil {
		recordServiceStatus("docker_error")
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer dc.Close()

	started := time.Now()
	targetService, _, err := dc.ServiceInspectWithRaw(ctx, service.ServiceName, swarm.ServiceInspectOptions{})
	observeDockerAPI("service_inspect", started, err)
	if err != nil {
		recordServiceStatus("service_missing")
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "docker service could not be retrieved or non existent"})
		return
	}

	recordServiceStatus("ok")
	ctx.JSON(http.StatusOK, gin.H{
		"service": service.ServiceName,
		"image":   targetService.Spec.TaskTemplate.ContainerSpec.Image,
	})
}

func serviceUpdate(ctx *gin.Context) {
	started := time.Now()
	defer observeServiceUpdateDuration(started)

	var body ServiceUpdateRequest
	if err := ctx.ShouldBindJSON(&body); err != nil {
		recordServiceUpdate("bad_request", false)
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if !isValidTokenFormat(body.Token) {
		recordTokenLookup("invalid_format")
		recordServiceUpdate("bad_request", body.StartFirst)
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid token format"})
		return
	}

	if !isAllowedStopSignal(&body.StopSignal) {
		recordServiceUpdate("invalid_stop_signal", body.StartFirst)
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "stop signal not valid"})
		return
	}

	service, err := findServiceByToken(body.Token)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			recordTokenLookup("not_found")
			recordServiceUpdate("unauthorized", body.StartFirst)
			ctx.JSON(http.StatusNotFound, gin.H{"error": "token not found"})
			return
		}
		recordTokenLookup("error")
		recordServiceUpdate("error", body.StartFirst)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	recordTokenLookup("found")

	dc, err := newDockerClient()
	if err != nil {
		recordServiceUpdate("docker_error", body.StartFirst)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer dc.Close()

	inspectStarted := time.Now()
	targetService, _, err := dc.ServiceInspectWithRaw(ctx, service.ServiceName, swarm.ServiceInspectOptions{})
	observeDockerAPI("service_inspect", inspectStarted, err)
	if err != nil {
		recordServiceUpdate("service_missing", body.StartFirst)
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "docker service could not be retrieved or non existent"})
		return
	}

	oldRepository, _ := repoAndTagFromImage(targetService.Spec.TaskTemplate.ContainerSpec.Image)
	targetService.Spec.TaskTemplate.ContainerSpec.Image = oldRepository + body.Tag
	targetService.Spec.TaskTemplate.ContainerSpec.StopSignal = body.StopSignal
	targetService.Spec.UpdateConfig = &swarm.UpdateConfig{
		FailureAction: swarm.UpdateFailureActionRollback,
	}
	if body.StartFirst {
		targetService.Spec.UpdateConfig.Order = swarm.UpdateOrderStartFirst
	}

	// Filter out existing FLOCKMAN_* env vars to avoid accumulation
	env := filterEnvVars(targetService.Spec.TaskTemplate.ContainerSpec.Env, "FLOCKMAN_")
	env = append(env, "FLOCKMAN_IMAGE_TAG="+body.Tag, "FLOCKMAN_IMAGE_REPO="+oldRepository)
	targetService.Spec.TaskTemplate.ContainerSpec.Env = env

	updateStarted := time.Now()
	_, err = dc.ServiceUpdate(ctx, targetService.ID, targetService.Version, targetService.Spec, swarm.ServiceUpdateOptions{})
	observeDockerAPI("service_update", updateStarted, err)
	if err != nil {
		recordServiceUpdate("docker_error", body.StartFirst)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	recordServiceUpdate("ok", body.StartFirst)
	ctx.JSON(http.StatusOK, gin.H{
		"service": service.ServiceName,
		"image":   targetService.Spec.TaskTemplate.ContainerSpec.Image,
	})
}

// filterEnvVars removes environment variables that start with the given prefix
func filterEnvVars(env []string, prefix string) []string {
	filtered := make([]string, 0, len(env))
	for _, e := range env {
		if !strings.HasPrefix(e, prefix) {
			filtered = append(filtered, e)
		}
	}
	return filtered
}

func isValidTokenFormat(token string) bool {
	if len(token) != DefaultSize {
		return false
	}
	for _, c := range token {
		if !strings.ContainsRune(nanoid.DefaultAlphabet, c) {
			return false
		}
	}
	return true
}

func isAllowedStopSignal(signal *string) bool {
	if *signal == "" {
		*signal = DefaultStopSignal
		return true
	}
	return slices.Contains(AllowedStopSignals[:], *signal)
}

func repoAndTagFromImage(image string) (repo string, tag string) {
	// Remove digest if present (everything after @)
	if idx := strings.Index(image, "@"); idx != -1 {
		image = image[:idx]
	}

	lastColon := strings.LastIndex(image, ":")
	lastSlash := strings.LastIndex(image, "/")

	// If colon comes before slash, it's part of the registry, not the tag
	if lastColon == -1 || lastColon < lastSlash {
		return image + ":", ""
	}

	return image[:lastColon+1], image[lastColon+1:]
}
