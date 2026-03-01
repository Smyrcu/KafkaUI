package handlers

import (
	_ "embed"
	"log/slog"
	"net/http"
)

//go:embed openapi.yaml
var openAPISpec []byte

func SwaggerSpec(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/yaml")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	if _, err := w.Write(openAPISpec); err != nil {
		slog.Error("failed to write OpenAPI spec", "error", err)
	}
}

func SwaggerUI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if _, err := w.Write([]byte(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>KafkaUI API Documentation</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css">
  <style>
    body { margin: 0; }
    .topbar { display: none; }
  </style>
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
  <script>
    SwaggerUIBundle({
      url: '/api/v1/docs/openapi.yaml',
      dom_id: '#swagger-ui',
      deepLinking: true,
      presets: [SwaggerUIBundle.presets.apis, SwaggerUIBundle.SwaggerUIStandalonePreset],
      layout: 'BaseLayout',
    });
  </script>
</body>
</html>`)); err != nil {
		slog.Error("failed to write Swagger UI", "error", err)
	}
}
