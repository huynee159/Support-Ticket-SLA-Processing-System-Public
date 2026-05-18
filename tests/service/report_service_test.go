package service_test

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	domain "support-ticket.com/internal/model"
	"support-ticket.com/internal/service"
	testmock "support-ticket.com/tests/mock"
)

func TestReportService_GenerateReport(t *testing.T) {
	now := time.Now()
	validReport := &domain.TicketReport{ReportDate: now}
	invalidReport := &domain.TicketReport{ReportDate: now, NewCount: -1} // Invalid count

	tests := []struct {
		name          string
		mockRepo      func(*testmock.MockReportRepository)
		expectedError string
		expectedRes   *domain.TicketReport
	}{
		{
			name: "Success",
			mockRepo: func(m *testmock.MockReportRepository) {
				m.On("AggregateByDate", now).Return(validReport, nil)
				m.On("Upsert", validReport).Return(nil)
			},
			expectedError: "",
			expectedRes:   validReport,
		},
		{
			name: "AggregateError",
			mockRepo: func(m *testmock.MockReportRepository) {
				m.On("AggregateByDate", now).Return((*domain.TicketReport)(nil), errors.New("db error"))
			},
			expectedError: "aggregate report",
			expectedRes:   nil,
		},
		{
			name: "UpsertError",
			mockRepo: func(m *testmock.MockReportRepository) {
				m.On("AggregateByDate", now).Return(validReport, nil)
				m.On("Upsert", validReport).Return(errors.New("db error"))
			},
			expectedError: "save report",
			expectedRes:   nil,
		},
		{
			name: "ValidationError",
			mockRepo: func(m *testmock.MockReportRepository) {
				m.On("AggregateByDate", now).Return(invalidReport, nil)
			},
			expectedError: "invalid report data",
			expectedRes:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := new(testmock.MockReportRepository)
			tt.mockRepo(mockRepo)

			svc := service.NewReportService(mockRepo)
			res, err := svc.GenerateReport(now)

			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
				assert.Nil(t, res)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedRes, res)
			}

			mockRepo.AssertExpectations(t)
		})
	}
}

func TestReportService_GetReport(t *testing.T) {
	now := time.Now()
	validReport := &domain.TicketReport{ReportDate: now}

	tests := []struct {
		name          string
		mockRepo      func(*testmock.MockReportRepository)
		expectedError string
		expectedRes   *domain.TicketReport
	}{
		{
			name: "Success",
			mockRepo: func(m *testmock.MockReportRepository) {
				m.On("GetByDate", now).Return(validReport, nil)
			},
			expectedError: "",
			expectedRes:   validReport,
		},
		{
			name: "Error",
			mockRepo: func(m *testmock.MockReportRepository) {
				m.On("GetByDate", now).Return((*domain.TicketReport)(nil), errors.New("not found"))
			},
			expectedError: "not found",
			expectedRes:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := new(testmock.MockReportRepository)
			tt.mockRepo(mockRepo)

			svc := service.NewReportService(mockRepo)
			res, err := svc.GetReport(now)

			if tt.expectedError != "" {
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
