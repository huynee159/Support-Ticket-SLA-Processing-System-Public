package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"support-ticket.com/internal/dto/common"
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

func TestTicketCreate(t *testing.T) {
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		mockRepo := new(MockTicketRepository)
		mockEventRepo := new(MockTicketEventRepository)
		svc := NewTicketService(mockRepo, mockEventRepo)
		dueAt := time.Now().Add(2 * time.Hour)
		req := request.CreateTicketReq{
			RequestorID: "user1",
			Title:       "Test Ticket",
			Description: "Description",
			Priority:    domain.PriorityHigh,
			SlaDueAt:    &dueAt,
		}

		mockRepo.On("Create", ctx, mock.AnythingOfType("*domain.Ticket")).Return(nil)

		res, err := svc.Create(ctx, req)

		assert.NoError(t, err)
		assert.NotNil(t, res)
		assert.Equal(t, "Test Ticket", res.Title)
		assert.Equal(t, domain.StatusNew, res.Status)
		assert.NotNil(t, res.SLADueAt)
		mockRepo.AssertExpectations(t)
	})

	t.Run("DBError", func(t *testing.T) {
		mockRepo := new(MockTicketRepository)
		mockEventRepo := new(MockTicketEventRepository)
		svc := NewTicketService(mockRepo, mockEventRepo)
		dueAt := time.Now().Add(2 * time.Hour)
		req := request.CreateTicketReq{
			RequestorID: "user1",
			Title:       "Test Ticket",
			Priority:    domain.PriorityHigh,
			SlaDueAt:    &dueAt,
		}
		mockRepo.On("Create", ctx, mock.Anything).Return(errors.New("db error"))

		res, err := svc.Create(ctx, req)

		assert.Error(t, err)
		assert.Nil(t, res)
	})
}

func TestFindById(t *testing.T) {
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		mockRepo := new(MockTicketRepository)
		mockEventRepo := new(MockTicketEventRepository)
		svc := NewTicketService(mockRepo, mockEventRepo)
		ticket := &domain.Ticket{ID: 1, Title: "Test"}
		mockRepo.On("FindById", ctx, uint(1)).Return(ticket, nil)

		res, err := svc.FindById(ctx, 1)

		assert.NoError(t, err)
		assert.Equal(t, ticket, res)
	})

	t.Run("NotFound", func(t *testing.T) {
		mockRepo := new(MockTicketRepository)
		mockEventRepo := new(MockTicketEventRepository)
		svc := NewTicketService(mockRepo, mockEventRepo)
		mockRepo.On("FindById", ctx, uint(1)).Return(nil, nil)

		res, err := svc.FindById(ctx, 1)

		assert.Error(t, err)
		assert.Nil(t, res)
	})
}


func TestUpdateTicketStatus(t *testing.T) {
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		mockRepo := new(MockTicketRepository)
		mockEventRepo := new(MockTicketEventRepository)
		svc := NewTicketService(mockRepo, mockEventRepo)
		ticket := &domain.Ticket{
			ID:         1,
			Status:     domain.StatusAssigned,
			AssigneeID: "agent1",
		}
		req := request.UpdateStatusReq{
			Status:     domain.StatusInProgress,
			AssigneeID: "agent1",
		}

		mockRepo.On("FindById", ctx, uint(1)).Return(ticket, nil)
		mockRepo.On("UpdateStatusWithEvent", ctx, mock.Anything, mock.Anything).Return(nil)

		err := svc.UpdateTicketStatus(ctx, 1, req)

		assert.NoError(t, err)
		mockRepo.AssertExpectations(t)
	})

}

func TestFindAll(t *testing.T) {
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		mockRepo := new(MockTicketRepository)
		mockEventRepo := new(MockTicketEventRepository)
		svc := NewTicketService(mockRepo, mockEventRepo)
		tickets := []domain.Ticket{{ID: 1, Title: "Test"}}
		filter := request.TicketFilter{}
		paging := common.PaginationQuery{Page: 1, Limit: 10}

		mockRepo.On("FindAll", ctx, filter, 0, 10).Return(tickets, int64(1), nil)

		res, err := svc.FindAll(ctx, filter, paging)

		assert.NoError(t, err)
		assert.Equal(t, tickets, res.Items)
		assert.Equal(t, int64(1), res.Total)
		assert.Equal(t, 1, res.Page)
	})
}


