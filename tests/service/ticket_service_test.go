package service_test

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
	"support-ticket.com/internal/service"
	testmock "support-ticket.com/tests/mock"
)

func TestTicketService_Create(t *testing.T) {
	ctx := context.Background()
	dueAt := time.Now().Add(2 * time.Hour)

	tests := []struct {
		name          string
		req           request.CreateTicketReq
		mockRepo      func(*testmock.MockTicketRepository)
		expectedError string
	}{
		{
			name: "Success",
			req: request.CreateTicketReq{
				RequestorID: "user1",
				Title:       "Test Ticket",
				Description: "Description",
				Priority:    domain.PriorityHigh,
				SlaDueAt:    &dueAt,
			},
			mockRepo: func(m *testmock.MockTicketRepository) {
				m.On("Create", ctx, mock.AnythingOfType("*domain.Ticket")).Return(nil)
			},
		},
		{
			name: "DBError",
			req: request.CreateTicketReq{
				RequestorID: "user1",
				Title:       "Test Ticket",
				Description: "Description",
				Priority:    domain.PriorityHigh,
				SlaDueAt:    &dueAt,
			},
			mockRepo: func(m *testmock.MockTicketRepository) {
				m.On("Create", ctx, mock.Anything).Return(errors.New("db error"))
			},
			expectedError: "db error",
		},
		{
			name: "ValidationError",
			req: request.CreateTicketReq{
				RequestorID: "user1",
				Description: "Description",
				Priority:    domain.PriorityHigh,
				SlaDueAt:    &dueAt,
			},
			mockRepo:      func(m *testmock.MockTicketRepository) {},
			expectedError: "Title is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := new(testmock.MockTicketRepository)
			mockEventRepo := new(testmock.MockTicketEventRepository)
			tt.mockRepo(mockRepo)

			svc := service.NewTicketService(mockRepo, mockEventRepo)
			res, err := svc.Create(ctx, tt.req)

			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
				assert.Nil(t, res)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, res)
				assert.Equal(t, tt.req.Title, res.Title)
				assert.Equal(t, domain.StatusNew, res.Status)
				assert.NotNil(t, res.SLADueAt)
			}
			mockRepo.AssertExpectations(t)
		})
	}
}

func TestTicketService_FindById(t *testing.T) {
	ctx := context.Background()
	ticket := &domain.Ticket{ID: 1, Title: "Test"}

	tests := []struct {
		name          string
		id            uint
		mockRepo      func(*testmock.MockTicketRepository)
		expectedRes   *domain.Ticket
		expectedError bool
	}{
		{
			name: "Success",
			id:   1,
			mockRepo: func(m *testmock.MockTicketRepository) {
				m.On("FindById", ctx, uint(1)).Return(ticket, nil)
			},
			expectedRes:   ticket,
			expectedError: false,
		},
		{
			name: "NotFound",
			id:   1,
			mockRepo: func(m *testmock.MockTicketRepository) {
				m.On("FindById", ctx, uint(1)).Return((*domain.Ticket)(nil), nil)
			},
			expectedRes:   nil,
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := new(testmock.MockTicketRepository)
			mockEventRepo := new(testmock.MockTicketEventRepository)
			tt.mockRepo(mockRepo)

			svc := service.NewTicketService(mockRepo, mockEventRepo)
			res, err := svc.FindById(ctx, tt.id)

			if tt.expectedError {
				assert.Error(t, err)
				assert.Nil(t, res)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedRes, res)
			}
			mockRepo.AssertExpectations(t)
		})
	}
}

func TestTicketService_UpdateTicketStatus(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name          string
		id            uint
		req           request.UpdateStatusReq
		mockRepo      func(*testmock.MockTicketRepository)
		expectedError string
	}{
		{
			name: "Success",
			id:   1,
			req: request.UpdateStatusReq{
				Status:     domain.StatusInProgress,
				AssigneeID: "agent1",
			},
			mockRepo: func(m *testmock.MockTicketRepository) {
				ticket := &domain.Ticket{
					ID:         1,
					Status:     domain.StatusAssigned,
					AssigneeID: "agent1",
				}
				m.On("FindById", ctx, uint(1)).Return(ticket, nil)
				m.On("UpdateStatusWithEvent", ctx, mock.Anything, mock.Anything).Return(nil)
			},
			expectedError: "",
		},
		{
			name: "ValidationError_InvalidTransition",
			id:   1,
			req: request.UpdateStatusReq{
				Status:     domain.StatusInProgress,
				AssigneeID: "agent1",
			},
			mockRepo: func(m *testmock.MockTicketRepository) {
				ticket := &domain.Ticket{
					ID:         1,
					Status:     domain.StatusNew,
					AssigneeID: "agent1",
				}
				m.On("FindById", ctx, uint(1)).Return(ticket, nil)
			},
			expectedError: "Cannot transition from 'new' to 'in_progress'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := new(testmock.MockTicketRepository)
			mockEventRepo := new(testmock.MockTicketEventRepository)
			tt.mockRepo(mockRepo)

			svc := service.NewTicketService(mockRepo, mockEventRepo)
			err := svc.UpdateTicketStatus(ctx, tt.id, tt.req)

			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				assert.NoError(t, err)
			}
			mockRepo.AssertExpectations(t)
		})
	}
}

func TestTicketService_FindAll(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name        string
		filter      request.TicketFilter
		paging      common.PaginationQuery
		mockRepo    func(*testmock.MockTicketRepository)
		expectedRes *common.PaginatedResult[domain.Ticket]
	}{
		{
			name:   "Success",
			filter: request.TicketFilter{},
			paging: common.PaginationQuery{Page: 1, Limit: 10},
			mockRepo: func(m *testmock.MockTicketRepository) {
				tickets := []domain.Ticket{{ID: 1, Title: "Test"}}
				m.On("FindAll", ctx, request.TicketFilter{}, 0, 10).Return(tickets, int64(1), nil)
			},
			expectedRes: &common.PaginatedResult[domain.Ticket]{
				Items:      []domain.Ticket{{ID: 1, Title: "Test"}},
				Total:      1,
				Page:       1,
				Limit:      10,
				TotalPages: 1,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := new(testmock.MockTicketRepository)
			mockEventRepo := new(testmock.MockTicketEventRepository)
			tt.mockRepo(mockRepo)

			svc := service.NewTicketService(mockRepo, mockEventRepo)
			res, err := svc.FindAll(ctx, tt.filter, tt.paging)

			assert.NoError(t, err)
			assert.Equal(t, tt.expectedRes.Items, res.Items)
			assert.Equal(t, tt.expectedRes.Total, res.Total)
			assert.Equal(t, tt.expectedRes.Page, res.Page)
			mockRepo.AssertExpectations(t)
		})
	}
}
