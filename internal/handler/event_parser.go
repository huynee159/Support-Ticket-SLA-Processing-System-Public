package handler

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"support-ticket.com/internal/dto/common"
	"support-ticket.com/internal/errmsgs"
	domain "support-ticket.com/internal/model"
)

// csvRequiredColumns are the mandatory headers expected in every CSV import.
var csvRequiredColumns = []string{"ticket_id", "from_status", "to_status", "assignee_id", "created_at"}

// parseEvents dispatches to the correct parser based on format ("json" or "csv")
// and returns the parsed events ready to pass to the service.
func parseEvents(data []byte, format string) ([]domain.TicketEvent, error) {
	if len(data) == 0 {
		return nil, errmsgs.ErrEmptyBody
	}
	switch strings.ToLower(format) {
	case "json":
		return parseJSON(data)
	case "csv":
		return parseCSV(data)
	default:
		return nil, errmsgs.ErrUnsupportedFileFormat
	}
}

// parseJSON deserializes a JSON array of TicketEvent.
func parseJSON(data []byte) ([]domain.TicketEvent, error) {
	var events []domain.TicketEvent
	if err := json.Unmarshal(data, &events); err != nil {
		return nil, common.NewBadRequest(common.ErrCodeInvalidBody, "invalid JSON: "+err.Error())
	}
	return events, nil
}

// parseCSV deserializes a CSV file into a slice of TicketEvent.
// Expected header (case-insensitive): ticket_id, from_status, to_status, assignee_id, created_at[, note]
func parseCSV(data []byte) ([]domain.TicketEvent, error) {
	records, err := csv.NewReader(bytes.NewReader(data)).ReadAll()
	if err != nil {
		return nil, common.NewBadRequest(common.ErrCodeInvalidBody, "invalid CSV: "+err.Error())
	}
	if len(records) < 2 {
		return nil, nil // header-only or empty → service will return ErrEmptyBatch
	}

	colIndex, err := buildColIndex(records[0])
	if err != nil {
		return nil, err
	}

	events := make([]domain.TicketEvent, 0, len(records)-1)
	for i, row := range records[1:] {
		event, err := parseCSVRow(row, colIndex, i+2)
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, nil
}

// buildColIndex builds a case-insensitive column-name→index map and validates
// that all required columns are present.
func buildColIndex(header []string) (map[string]int, error) {
	colIndex := make(map[string]int, len(header))
	for i, h := range header {
		colIndex[strings.ToLower(strings.TrimSpace(h))] = i
	}
	for _, required := range csvRequiredColumns {
		if _, ok := colIndex[required]; !ok {
			return nil, common.NewBadRequest(common.ErrCodeInvalidBody,
				fmt.Sprintf("invalid CSV: missing required column '%s'", required))
		}
	}
	return colIndex, nil
}

// parseCSVRow converts a single CSV row into a TicketEvent.
// rowNum is 1-indexed and used only for error messages.
func parseCSVRow(row []string, colIndex map[string]int, rowNum int) (domain.TicketEvent, error) {
	if len(row) < len(colIndex) {
		return domain.TicketEvent{}, common.NewBadRequest(common.ErrCodeInvalidBody,
			fmt.Sprintf("invalid CSV: row %d has fewer columns than header", rowNum))
	}

	ticketIDStr := strings.TrimSpace(row[colIndex["ticket_id"]])
	ticketIDRaw, err := strconv.ParseUint(ticketIDStr, 10, 64)
	if err != nil {
		return domain.TicketEvent{}, common.NewBadRequest(common.ErrCodeInvalidInput,
			fmt.Sprintf("invalid CSV: row %d — ticket_id '%s' is not a valid integer", rowNum, ticketIDStr))
	}

	createdAt, err := parseCSVTime(strings.TrimSpace(row[colIndex["created_at"]]))
	if err != nil {
		return domain.TicketEvent{}, common.NewBadRequest(common.ErrCodeInvalidInput,
			fmt.Sprintf("invalid CSV: row %d — created_at '%s' must be RFC3339 or 'YYYY-MM-DD HH:MM:SS'",
				rowNum, row[colIndex["created_at"]]))
	}

	var note *string
	if idx, ok := colIndex["note"]; ok {
		if v := strings.TrimSpace(row[idx]); v != "" {
			note = &v
		}
	}

	return domain.TicketEvent{
		TicketID:   uint(ticketIDRaw),
		FromStatus: domain.TicketStatus(strings.TrimSpace(row[colIndex["from_status"]])),
		ToStatus:   domain.TicketStatus(strings.TrimSpace(row[colIndex["to_status"]])),
		AssigneeID: strings.TrimSpace(row[colIndex["assignee_id"]]),
		CreatedAt:  createdAt,
		Note:       note,
	}, nil
}

// parseCSVTime tries RFC3339 first, then falls back to "YYYY-MM-DD HH:MM:SS".
func parseCSVTime(s string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	return time.Parse("2006-01-02 15:04:05", s)
}
