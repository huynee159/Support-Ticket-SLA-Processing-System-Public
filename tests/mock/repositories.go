package mock

import (
	"context"
	"time"

	"github.com/stretchr/testify/mock"
	"support-ticket.com/internal/dto/request"
	domain "support-ticket.com/internal/model"
)

// MockTicketRepository
type MockTicketRepository struct {
	mock.Mock
}

func (m *MockTicketRepository) Create(ctx context.Context, ticket *domain.Ticket) error {
	args := m.Called(ctx, ticket)
	return args.Error(0)
}

func (m *MockTicketRepository) FindById(ctx context.Context, id uint) (*domain.Ticket, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Ticket), args.Error(1)
}

func (m *MockTicketRepository) FindAll(ctx context.Context, filter request.TicketFilter, offset, limit int) ([]domain.Ticket, int64, error) {
	args := m.Called(ctx, filter, offset, limit)
	if args.Get(0) == nil {
		return nil, args.Get(1).(int64), args.Error(2)
	}
	return args.Get(0).([]domain.Ticket), args.Get(1).(int64), args.Error(2)
}

func (m *MockTicketRepository) UpdateStatusWithEvent(ctx context.Context, ticket *domain.Ticket, event *domain.TicketEvent) error {
	args := m.Called(ctx, ticket, event)
	return args.Error(0)
}

func (m *MockTicketRepository) GetExistingTicketIDs(ctx context.Context, ticketIDs []uint) (map[uint]bool, error) {
	args := m.Called(ctx, ticketIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[uint]bool), args.Error(1)
}

func (m *MockTicketRepository) GetTicketStatusAndCreatedAt(ctx context.Context, ticketIDs []uint) (map[uint]domain.TicketStatus, map[uint]time.Time, error) {
	args := m.Called(ctx, ticketIDs)
	if args.Get(0) == nil {
		return nil, nil, args.Error(2)
	}
	return args.Get(0).(map[uint]domain.TicketStatus), args.Get(1).(map[uint]time.Time), args.Error(2)
}

func (m *MockTicketRepository) UpdateStatusAndAssignee(ctx context.Context, ticketID uint, status domain.TicketStatus, assigneeID string) error {
	args := m.Called(ctx, ticketID, status, assigneeID)
	return args.Error(0)
}

func (m *MockTicketRepository) UpdateStatusAndResolvedAt(ctx context.Context, ticketID uint, status domain.TicketStatus, assigneeID string, resolvedAt time.Time) error {
	args := m.Called(ctx, ticketID, status, assigneeID, resolvedAt)
	return args.Error(0)
}

func (m *MockTicketRepository) UpdateStatusAndCancelledAt(ctx context.Context, ticketID uint, status domain.TicketStatus, assigneeID string, cancelledAt time.Time) error {
	args := m.Called(ctx, ticketID, status, assigneeID, cancelledAt)
	return args.Error(0)
}

// MockTicketEventRepository
type MockTicketEventRepository struct {
	mock.Mock
}

func (m *MockTicketEventRepository) CreateBatch(events []domain.TicketEvent) error {
	args := m.Called(events)
	return args.Error(0)
}

func (m *MockTicketEventRepository) Create(ctx context.Context, event *domain.TicketEvent) error {
	args := m.Called(ctx, event)
	return args.Error(0)
}

func (m *MockTicketEventRepository) GetExistingEventKeys(ctx context.Context, keys []string) (map[string]bool, error) {
	args := m.Called(ctx, keys)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]bool), args.Error(1)
}

func (m *MockTicketEventRepository) FetchLatestEventPerTicket(ctx context.Context, ticketIDs []int) ([]domain.TicketEvent, error) {
	args := m.Called(ctx, ticketIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]domain.TicketEvent), args.Error(1)
}

func (m *MockTicketEventRepository) FetchLatestResolvedEventPerTicket(ctx context.Context, ticketIDs []int) ([]domain.TicketEvent, error) {
	args := m.Called(ctx, ticketIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]domain.TicketEvent), args.Error(1)
}

// MockReportRepository
type MockReportRepository struct {
	mock.Mock
}

func (m *MockReportRepository) AggregateByDate(date time.Time) (*domain.TicketReport, error) {
	args := m.Called(date)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.TicketReport), args.Error(1)
}

func (m *MockReportRepository) Upsert(report *domain.TicketReport) error {
	args := m.Called(report)
	return args.Error(0)
}

func (m *MockReportRepository) GetByDate(date time.Time) (*domain.TicketReport, error) {
	args := m.Called(date)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.TicketReport), args.Error(1)
}
