package domain

import (
	"fmt"
	"strings"
	"time"

	"support-ticket.com/internal/dto/common"
	"support-ticket.com/internal/errmsgs"
)

type TicketStatus string
type Priority string

const (
	StatusNew        TicketStatus = "new"
	StatusAssigned   TicketStatus = "assigned"
	StatusInProgress TicketStatus = "in_progress"
	StatusResolved   TicketStatus = "resolved"
	StatusClosed     TicketStatus = "closed"
	StatusCancelled  TicketStatus = "cancelled"
)

const (
	PriorityLow    Priority = "low"
	PriorityMedium Priority = "medium"
	PriorityHigh   Priority = "high"
)

type Ticket struct {
	ID          uint         `json:"id" gorm:"primaryKey"`
	AssigneeID  string       `json:"assignee_id" gorm:"column:assignee_id;type:varchar(255)"`
	RequestorID string       `json:"requestor_id" gorm:"column:requestor_id;type:varchar(255);not null"`
	Title       string       `json:"title" gorm:"column:title;type:varchar(255);not null"`
	Description string       `json:"description" gorm:"column:description;type:text"`
	Priority    Priority     `json:"priority" gorm:"column:priority;type:varchar(20);not null"`
	Status      TicketStatus `json:"status" gorm:"column:status;type:varchar(20);not null"`
	CreatedAt   time.Time    `json:"created_at" gorm:"column:created_at;not null;autoCreateTime:milli"`
	ResolvedAt  *time.Time   `json:"resolved_at" gorm:"column:resolved_at"`
	SLADueAt    *time.Time   `json:"sla_due_at" gorm:"column:sla_due_at"`
	CancelledAt *time.Time   `json:"cancelled_at" gorm:"column:cancelled_at"`

	// TODO:Relations
	Events []TicketEvent `json:"events" gorm:"foreignKey:TicketID;constraint:OnDelete:CASCADE"`
}

func (p Priority) IsValid() bool {
	switch p {
	case PriorityLow, PriorityMedium, PriorityHigh:
		return true
	}
	return false
}

func (s TicketStatus) IsValid() bool {
	switch s {
	case StatusNew, StatusAssigned, StatusInProgress, StatusResolved, StatusClosed, StatusCancelled:
		return true
	}
	return false
}

var ticketTransitions = map[TicketStatus]map[TicketStatus]bool{
	StatusNew: {
		StatusAssigned:  true,
		StatusCancelled: true,
	},
	StatusAssigned: {
		StatusInProgress: true,
		StatusCancelled:  true,
	},
	StatusInProgress: {
		StatusResolved: true,
	},
	StatusResolved: {
		StatusClosed: true,
	},
}

func (s TicketStatus) CanTransitionTo(next TicketStatus) bool {
	allowed, ok := ticketTransitions[s]
	if !ok {
		return false
	}
	return allowed[next]
}

func (t *Ticket) Validate() error {
	if strings.TrimSpace(t.Title) == "" {
		return common.NewBadRequest(common.ErrCodeInvalidInput, "title is required")
	}
	if strings.TrimSpace(t.Description) == "" {
		return common.NewBadRequest(common.ErrCodeInvalidInput, "description is required")
	}
	if strings.TrimSpace(t.RequestorID) == "" {
		return common.NewBadRequest(common.ErrCodeInvalidInput, "requestor_id is required")
	}
	if !t.Priority.IsValid() {
		return common.NewBadRequest(common.ErrCodeInvalidInput, fmt.Sprintf("unknown priority '%s'", t.Priority))
	}
	if !t.Status.IsValid() {
		return common.NewBadRequest(common.ErrCodeInvalidInput, fmt.Sprintf("unknown status '%s'", t.Status))
	}
	if t.CreatedAt.IsZero() {
		return common.NewBadRequest(common.ErrCodeInvalidInput, "created_at is required")
	}
	if t.SLADueAt == nil || t.SLADueAt.IsZero() {
		return common.NewBadRequest(common.ErrCodeInvalidInput, "sla_due_at is required for SLA tracking")
	}
	if t.SLADueAt.Before(t.CreatedAt) {
		return common.NewBadRequest(common.ErrCodeInvalidInput, "sla_due_at cannot be before the ticket creation time")
	}
	if t.Status == StatusResolved {
		if err := validateTimestampAfterCreation(t.ResolvedAt, "resolved_at", t.CreatedAt); err != nil {
			return err
		}
	}
	if t.Status == StatusCancelled {
		if err := validateTimestampAfterCreation(t.CancelledAt, "cancelled_at", t.CreatedAt); err != nil {
			return err
		}
	}
	return nil
}

func validateTimestampAfterCreation(ts *time.Time, field string, createdAt time.Time) error {
	if ts == nil || ts.IsZero() {
		return common.NewBadRequest(common.ErrCodeInvalidInput, fmt.Sprintf("%s is required", field))
	}
	if ts.Before(createdAt) {
		return common.NewBadRequest(common.ErrCodeInvalidInput, fmt.Sprintf("%s cannot be before created_at", field))
	}
	return nil
}

func (t *Ticket) ValidateStatusTransition(newStatus TicketStatus, reqAssigneeId string, timestamp time.Time) error {
	reqAssigneeId = strings.TrimSpace(reqAssigneeId)

	if t.Status == StatusNew && newStatus == StatusAssigned {
		if reqAssigneeId == "" {
			return common.NewBadRequest(common.ErrCodeInvalidInput, "assignee_id is required when assigning a ticket")
		}
		t.AssigneeID = reqAssigneeId
	} else if reqAssigneeId != "" && reqAssigneeId != t.AssigneeID {
		return common.NewBadRequest(common.ErrCodeInvalidInput,
			fmt.Sprintf("cannot change assignee during status transition to '%s'",newStatus))
	}

	if t.Status == newStatus {
		return common.NewBadRequest(common.ErrCodeInvalidTransition,
			fmt.Sprintf("status is already set to '%s'", newStatus))
	}
	if !newStatus.IsValid() {
		return common.NewBadRequest(common.ErrCodeInvalidTransition,
			fmt.Sprintf("cannot transition to unknown status '%s'", newStatus))
	}
	if !t.Status.CanTransitionTo(newStatus) {
		return common.NewBadRequest(common.ErrCodeInvalidTransition,
			fmt.Sprintf("cannot transition from '%s' to '%s'", t.Status, newStatus))
	}

	switch newStatus {
	case StatusResolved:
		t.ResolvedAt = &timestamp
		if err := t.ValidateResolvedAt(t.CreatedAt); err != nil {
			return err
		}
	case StatusCancelled:
		t.CancelledAt = &timestamp
		if err := t.ValidateCancelledAt(t.CreatedAt); err != nil {
			return err
		}
	}
	return nil
}

func (t *Ticket) ValidateResolvedAt(createdAt time.Time) error {
	return validateTimestampAfterCreation(t.ResolvedAt, "resolved_at", createdAt)
}

func (t *Ticket) ValidateCancelledAt(createdAt time.Time) error {
	return validateTimestampAfterCreation(t.CancelledAt, "cancelled_at", createdAt)
}

var _ = errmsgs.ErrInvalidInput
