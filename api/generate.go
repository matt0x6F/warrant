// Package api contains the OpenAPI spec and generated REST client and server.
// Regenerate after editing api/openapi.yaml:
//
//	go generate ./api/...
//
// Requires: go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest
package api

//go:generate oapi-codegen -config oapi-codegen.yaml openapi.yaml
//go:generate oapi-codegen -config oapi-codegen-server.yaml openapi.yaml
