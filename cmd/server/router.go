package main

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	_ "irrigation-server/docs"

	"irrigation-server/internal/features/asset"
	"irrigation-server/internal/features/canal"
	"irrigation-server/internal/features/health"
	"irrigation-server/internal/features/ingest"
	"irrigation-server/internal/features/plot"
	"irrigation-server/internal/features/quality"
	"irrigation-server/internal/pkg/apperror"
	"irrigation-server/internal/pkg/config"
	"irrigation-server/internal/pkg/middleware"
	"irrigation-server/internal/pkg/response"
	"irrigation-server/internal/pkg/scalar"
	"irrigation-server/internal/pkg/swagger"
)

var (
	errRouteNotFound    = apperror.NotFound("ROUTE_NOT_FOUND", "alamat tidak ditemukan")
	errMethodNotAllowed = apperror.MethodNotAllowed("METHOD_NOT_ALLOWED", "metode HTTP tidak didukung untuk alamat ini")
)

func newRouter(cfg config.Config, log *slog.Logger, pool *pgxpool.Pool, ingestSvc ingest.Service) *gin.Engine {
	if cfg.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	// Gin trusts every proxy by default, which lets clients forge X-Forwarded-For.
	_ = r.SetTrustedProxies(nil)
	// Otherwise a wrong method answers 404 instead of 405.
	r.HandleMethodNotAllowed = true
	r.Use(
		middleware.RequestID(),
		middleware.Recovery(log),
		middleware.Logger(log),
		middleware.Timeout(cfg.RequestTimeout),
	)
	r.NoRoute(func(c *gin.Context) { response.Error(c, errRouteNotFound) })
	r.NoMethod(func(c *gin.Context) { response.Error(c, errMethodNotAllowed) })

	r.GET("/", serviceIndex(cfg))
	if cfg.DocsEnabled {
		swagger.Register(r)
		scalar.Register(r, "/scalar", swagger.SpecPath, "Irrigation API Reference")
	}

	api := r.Group("/api")
	// Answers while the import runs.
	health.RegisterRoutes(api, health.NewHandler(ingestSvc))

	// Everything else waits for the import.
	ready := api.Group("", requireReady(ingestSvc))
	ingest.RegisterRoutes(ready, ingest.NewHandler(ingestSvc))
	asset.RegisterRoutes(ready, asset.NewHandler(asset.NewService(asset.NewRepository(pool))))
	canal.RegisterRoutes(ready, canal.NewHandler(canal.NewService(canal.NewRepository(pool))))
	plot.RegisterRoutes(ready, plot.NewHandler(plot.NewService(plot.NewRepository(pool))))
	quality.RegisterRoutes(ready, quality.NewHandler(quality.NewService(quality.NewRepository(pool))))

	return r
}

func requireReady(status health.StatusProvider) gin.HandlerFunc {
	return func(c *gin.Context) {
		switch status.Status() {
		case ingest.StatusReady:
			c.Next()
		case ingest.StatusFailed:
			response.Error(c, ingest.ErrImportFailed)
		default:
			response.Error(c, ingest.ErrNotReady)
		}
	}
}

func serviceIndex(cfg config.Config) gin.HandlerFunc {
	index := gin.H{
		"service": "irrigation-server",
		"api":     "/api",
		"health":  "/api/health",
	}
	if cfg.DocsEnabled {
		index["docs"] = gin.H{"swagger": "/swagger/index.html", "scalar": "/scalar", "openapi": swagger.SpecPath}
	}
	return func(c *gin.Context) { c.JSON(http.StatusOK, index) }
}
