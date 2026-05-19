package handler

import (
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	dto "support-ticket.com/internal/dto/common"
	"support-ticket.com/internal/dto/response"
	"support-ticket.com/internal/service"
)

type TicketEventHandler struct {
	service service.TicketEventService
}

func NewTicketEventHandler(service service.TicketEventService) *TicketEventHandler {
	return &TicketEventHandler{
		service: service,
	}
}

// readImportInput reads raw bytes and detects the file format from the request.
// Supports multipart file upload (CSV/JSON) and raw JSON body (backward compatible).
func readImportInput(c *gin.Context) (data []byte, format string, err error) {
	if strings.Contains(c.GetHeader("Content-Type"), "multipart/form-data") {
		file, header, ferr := c.Request.FormFile("file")
		if ferr != nil {
			return nil, "", dto.NewBadRequest(dto.ErrCodeInvalidBody, "missing or invalid 'file' field in multipart form")
		}
		defer file.Close()

		format = strings.ToLower(strings.TrimPrefix(filepath.Ext(header.Filename), "."))
		data, err = io.ReadAll(file)
		return
	}

	// Raw JSON body (backward compatible)
	defer c.Request.Body.Close()
	data, err = io.ReadAll(c.Request.Body)
	format = "json"
	return
}

// ImportEvents godoc
// @Summary Import ticket events
// @Description Import ticket events in batch. Accepts a multipart file upload (CSV or JSON) via the `file` field, or a raw JSON body.
// @Tags ticket-events
// @Accept multipart/form-data
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param file formData file false "CSV or JSON file to import"
// @Param request body []map[string]interface{} false "Raw JSON array (when not using file upload)"
// @Success 200 {object} common.APIResponse[response.TicketImportResponse]
// @Failure 400 {object} common.APIResponse[any]
// @Failure 500 {object} common.APIResponse[any]
// @Router /ticket-events/import [post]
func (h *TicketEventHandler) ImportEvents(c *gin.Context) {
	ctx := c.Request.Context()

	data, format, err := readImportInput(c)
	if err != nil {
		HandleError(c, err)
		return
	}

	events, err := parseEvents(data, format)
	if err != nil {
		HandleError(c, err)
		return
	}

	result, err := h.service.Import(ctx, events)
	if err != nil {
		HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, dto.APIResponse[interface{}]{
		Success: true,
		Data:    response.NewTicketImportResponse(result),
		Message: "import completed",
	})
}
