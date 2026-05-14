package handlers

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

type healthResp struct {
	Status string `json:"status" example:"ok"`
}

// Health handles GET /health
//
//	@Summary     Health check
//	@Description Returns {"status":"ok"} when the server is up.
//	@Tags        System
//	@Produce     json
//	@Success     200 {object} healthResp
//	@Router      /health [get]
func Health(c echo.Context) error {
	return c.JSON(http.StatusOK, healthResp{Status: "ok"})
}
