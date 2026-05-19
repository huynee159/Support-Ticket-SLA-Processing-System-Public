package service

import (
	"context"
	"fmt"
	"math"
	"time"

	"support-ticket.com/internal/dto/common"
	"support-ticket.com/internal/dto/request"
	"support-ticket.com/internal/errmsgs"
	domain "support-ticket.com/internal/model"
	"support-ticket.com/internal/repository"
)

type TicketService interface {
	Create(ctx context.Context, req request.CreateTicketReq) (*domain.Ticket, error)
	FindById(ctx context.Context, id uint) (*domain.Ticket, error)
	FindAll(ctx context.Context, filter request.TicketFilter, paging common.PaginationQuery) (*common.PaginatedResult[domain.Ticket], error)
	UpdateTicketStatus(ctx context.Context, id uint, req request.UpdateStatusReq) error
}

type ticketServiceImpl struct {
	repo      repository.TicketRepository
	eventRepo repository.TicketEventRepository
}

func NewTicketService(repo repository.TicketRepository, eventRepo repository.TicketEventRepository) TicketService {
	return &ticketServiceImpl{
		repo:      repo,
		eventRepo: eventRepo,
	}
}

func (s *ticketServiceImpl) Create(ctx context.Context, req request.CreateTicketReq) (*domain.Ticket, error) {
	now := time.Now()

	ticket := &domain.Ticket{
		RequestorID: req.RequestorID,
		Title:       req.Title,
		Description: req.Description,
		Priority:    req.Priority,
		SLADueAt:    req.SlaDueAt,
		Status:      domain.StatusNew,
		CreatedAt:   now,
	}

	if err := ticket.Validate(); err != nil {
		return nil, fmt.Errorf("invalid ticket data: %w", err)
	}

	// Persistence: DB layer
	if err := s.repo.Create(ctx, ticket); err != nil {
		return nil, fmt.Errorf("failed to create ticket in db: %w", err)
	}

	return ticket, nil
}

func (s *ticketServiceImpl) FindById(ctx context.Context, id uint) (*domain.Ticket, error) {
	ticket, err := s.repo.FindById(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get ticket from db: %w", err)
	}

	if ticket == nil {
		return nil, errmsgs.ErrTicketNotFound
	}

	return ticket, nil
}

func (s *ticketServiceImpl) FindAll(ctx context.Context, filter request.TicketFilter, paging common.PaginationQuery) (*common.PaginatedResult[domain.Ticket], error) {
	limit := paging.GetLimit()
	offset := paging.GetOffset()
	page := paging.GetPage()

	tickets, total, err := s.repo.FindAll(ctx, filter, offset, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to list tickets: %w", err)
	}
	if tickets == nil {
		tickets = []domain.Ticket{}
	}

	totalPages := int(math.Ceil(float64(total) / float64(limit)))

	result := &common.PaginatedResult[domain.Ticket]{
		Items:      tickets,
		Total:      total,
		Page:       page,
		Limit:      limit,
		TotalPages: totalPages,
	}

	return result, nil
}

func (s *ticketServiceImpl) UpdateTicketStatus(ctx context.Context, id uint, req request.UpdateStatusReq) error {
	ticket, err := s.repo.FindById(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to get ticket: %w", err)
	}

	if err := ticket.ValidateStatusTransition(req.Status, req.AssigneeID, time.Now()); err != nil {
		return fmt.Errorf("%w", err)
	}

	// Build event
	event := &domain.TicketEvent{
		TicketID:   ticket.ID,
		AssigneeID: req.AssigneeID,
		FromStatus: ticket.Status,
		ToStatus:   req.Status,
		CreatedAt:  time.Now(),
	}
	if req.Note != "" {
		event.Note = &req.Note
	}
	event.Validate()
	

	ticket.Status = req.Status
	if err := s.repo.UpdateStatusWithEvent(ctx, ticket, event); err != nil {
		return fmt.Errorf("failed to update ticket status with event: %w", err)
	}

	return nil
}
