// Package scalar serves the Scalar API Reference, which reads the Swaggo specification.
package scalar

import (
	"bytes"
	"html/template"
	"net/http"

	"github.com/gin-gonic/gin"
)

// Version is pinned so the documentation page does not change by accident.
const Version = "1.72.4"

var page = template.Must(template.New("scalar").Parse(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>{{.Title}}</title>
</head>
<body>
  <div id="app"></div>
  <script src="https://cdn.jsdelivr.net/npm/@scalar/api-reference@{{.Version}}"></script>
  <script>
    Scalar.createApiReference('#app', { url: {{.SpecURL}} })
  </script>
</body>
</html>`))

// Register mounts the Scalar page at the given path (e.g. /scalar).
func Register(r gin.IRoutes, path, specURL, title string) {
	var buf bytes.Buffer
	if err := page.Execute(&buf, map[string]string{
		"Title":   title,
		"Version": Version,
		"SpecURL": specURL,
	}); err != nil {
		panic(err)
	}
	html := buf.Bytes()

	r.GET(path, func(c *gin.Context) {
		c.Data(http.StatusOK, "text/html; charset=utf-8", html)
	})
}
