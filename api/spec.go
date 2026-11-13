// Package api embeds the OpenAPI document the portal follows.
package api

import _ "embed"

//go:embed openapi.yaml
var OpenAPI []byte
