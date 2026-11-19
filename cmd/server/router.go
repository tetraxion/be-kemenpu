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

// newRouter assembles the middleware and every feature. New features are registered here.
func newRouter(cfg config.Config, log *slog.Logger, pool *pgxpool.Pool, ingestSvc ingest.Service) *gin.Engine {
	if cfg.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	// Gin trusts every proxy by default, which lets any client forge its
	// address through X-Forwarded-For. Nothing here needs forwarded addresses.
	// The error is for unparsable addresses; nil has none.
	_ = r.SetTrustedProxies(nil)
	// Without this a wrong method on a known address answers 404, which hides
	// the real mistake (e.g. GET on the POST-only follow-up import).
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
	// The status probe answers at any time, including while the import runs.
	health.RegisterRoutes(api, health.NewHandler(ingestSvc))

	// Everything else reads imported data, so it waits for the import.
	ready := api.Group("", requireReady(ingestSvc))
	ingest.RegisterRoutes(ready, ingest.NewHandler(ingestSvc))
	asset.RegisterRoutes(ready, asset.NewHandler(asset.NewService(asset.NewRepository(pool))))
	canal.RegisterRoutes(ready, canal.NewHandler(canal.NewService(canal.NewRepository(pool))))
	plot.RegisterRoutes(ready, plot.NewHandler(plot.NewService(plot.NewRepository(pool))))
	quality.RegisterRoutes(ready, quality.NewHandler(quality.NewService(quality.NewRepository(pool))))

	return r
}

// requireReady answers 503 until the initial import has finished, and says
// so plainly when the import failed, so a client does not keep waiting.
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

// serviceIndex answers the bare address with where to go next. This service
// is the API only; the web interface is a separate service.
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
