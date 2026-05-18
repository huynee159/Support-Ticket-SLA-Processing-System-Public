package service_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	domain "support-ticket.com/internal/model"
	"support-ticket.com/internal/service"
	testmock "support-ticket.com/tests/mock"
)

func TestTicketEventService_Import(t *testing.T) {
	ctx := context.Background()
	now := time.Now()

	validEvent := domain.TicketEvent{
		TicketID:   1,
		FromStatus: domain.StatusNew,
		ToStatus:   domain.StatusAssigned,
		AssigneeID: "agent1",
		CreatedAt:  now,
	}

	invalidTransitionEvent := domain.TicketEvent{
		TicketID:   2,
		FromStatus: domain.StatusNew,
		ToStatus:   domain.StatusNew, // Invalid transition
		AssigneeID: "agent1",
		CreatedAt:  now,
	}

	nonExistentTicketEvent := domain.TicketEvent{
		TicketID:   999,
		FromStatus: domain.StatusNew,
		ToStatus:   domain.StatusAssigned,
		AssigneeID: "agent1",
		CreatedAt:  now,
	}

	tests := []struct {
		name                 string
		inputData            func() []byte
		mockRepo             func(*testmock.MockTicketRepository)
		mockEventRepo        func(*testmock.MockTicketEventRepository)
		expectedError        string
		expectedAccepted     int
		expectedRejected     int
		expectedRejectDetail string
	}{
		{
			name: "SuccessSimpleImport",
			inputData: func() []byte {
				data, _ := json.Marshal([]domain.TicketEvent{validEvent})
				return data
			},
			mockRepo: func(m *testmock.MockTicketRepository) {
				m.On("GetExistingTicketIDs", ctx, []uint{1}).Return(map[uint]bool{1: true}, nil)
				m.On("GetTicketStatusAndCreatedAt", ctx, []uint{1}).Return(
					map[uint]domain.TicketStatus{1: domain.StatusNew},
					map[uint]time.Time{1: now.Add(-1 * time.Hour)},
					nil,
				)
				m.On("UpdateStatusAndAssignee", ctx, uint(1), domain.StatusAssigned, "agent1").Return(nil)
			},
			mockEventRepo: func(m *testmock.MockTicketEventRepository) {
				m.On("GetExistingEventKeys", ctx, mock.Anything).Return(map[string]bool{}, nil)
				m.On("CreateBatch", mock.Anything).Return(nil)
			},
			expectedAccepted: 1,
			expectedRejected: 0,
		},
		{
			name: "InvalidJSON",
			inputData: func() []byte {
				return []byte("invalid json")
			},
			mockRepo:         func(m *testmock.MockTicketRepository) {},
			mockEventRepo:    func(m *testmock.MockTicketEventRepository) {},
			expectedError:    "invalid JSON",
			expectedAccepted: 0,
			expectedRejected: 0,
		},
		{
			name: "EmptyBatch",
			inputData: func() []byte {
				return []byte("[]")
			},
			mockRepo:         func(m *testmock.MockTicketRepository) {},
			mockEventRepo:    func(m *testmock.MockTicketEventRepository) {},
			expectedError:    "batch is empty",
			expectedAccepted: 0,
			expectedRejected: 0,
		},
		{
			name: "RejectedNonExistentTicket",
			inputData: func() []byte {
				data, _ := json.Marshal([]domain.TicketEvent{nonExistentTicketEvent})
				return data
			},
			mockRepo: func(m *testmock.MockTicketRepository) {
				m.On("GetExistingTicketIDs", ctx, []uint{999}).Return(map[uint]bool{}, nil)
				m.On("GetTicketStatusAndCreatedAt", ctx, []uint{999}).Return(
					map[uint]domain.TicketStatus{},
					map[uint]time.Time{},
					nil,
				)
			},
			mockEventRepo: func(m *testmock.MockTicketEventRepository) {
				m.On("GetExistingEventKeys", ctx, mock.Anything).Return(map[string]bool{}, nil)
			},
			expectedAccepted:     0,
			expectedRejected:     1,
			expectedRejectDetail: "does not exist in DB",
		},
		{
			name: "ValidationError",
			inputData: func() []byte {
				data, _ := json.Marshal([]domain.TicketEvent{invalidTransitionEvent})
				return data
			},
			mockRepo: func(m *testmock.MockTicketRepository) {
				m.On("GetExistingTicketIDs", ctx, []uint(nil)).Return(map[uint]bool{}, nil)
				m.On("GetTicketStatusAndCreatedAt", ctx, []uint(nil)).Return(
					map[uint]domain.TicketStatus{},
					map[uint]time.Time{},
					nil,
				)
			},
			mockEventRepo: func(m *testmock.MockTicketEventRepository) {
				m.On("GetExistingEventKeys", ctx, mock.Anything).Return(map[string]bool{}, nil)
			},
			expectedAccepted:     0,
			expectedRejected:     1,
			expectedRejectDetail: "From Status and To Status cannot be the same",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := new(testmock.MockTicketRepository)
			mockEventRepo := new(testmock.MockTicketEventRepository)
			tt.mockRepo(mockRepo)
			tt.mockEventRepo(mockEventRepo)

			svc := service.NewTicketEventService(mockEventRepo, mockRepo)
			res, err := svc.Import(ctx, tt.inputData())

			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
				assert.Empty(t, res)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, res)
				assert.Equal(t, tt.expectedAccepted, res.AcceptedCount)
				assert.Equal(t, tt.expectedRejected, res.RejectedCount)
				if tt.expectedRejectDetail != "" {
					assert.Contains(t, res.RejectedDetails[0].ErrorName, tt.expectedRejectDetail)
				}
			}

			mockRepo.AssertExpectations(t)
			mockEventRepo.AssertExpectations(t)
		})
	}
}
