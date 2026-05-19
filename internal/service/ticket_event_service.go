package service

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"support-ticket.com/internal/config"
	"support-ticket.com/internal/dto/common"
	"support-ticket.com/internal/errmsgs"
	domain "support-ticket.com/internal/model"
	"support-ticket.com/internal/repository"
	"support-ticket.com/internal/worker"
)

type TicketEventService interface {
	Import(ctx context.Context, events []domain.TicketEvent) (domain.BatchImportResult, error)
}

type ticketEventService struct {
	eventRepo  repository.TicketEventRepository
	ticketRepo repository.TicketRepository
}

func NewTicketEventService(eventRepo repository.TicketEventRepository, ticketRepo repository.TicketRepository) TicketEventService {
	return &ticketEventService{
		eventRepo:  eventRepo,
		ticketRepo: ticketRepo,
	}
}

type updateJob struct {
	TicketID    uint
	Status      domain.TicketStatus
	AssigneeID  string
	CreatedAt   time.Time
	ResolvedAt  *time.Time
	CancelledAt *time.Time
}

var maxBatchSize = config.GetBatchSize("MAX_BATCH_SIZE")

type parsedEvent struct {
	Event domain.TicketEvent
	Err   error // nil = valid
}

type ticketWorkerJob struct {
	TicketID uint
	Events   []domain.TicketEvent
}

type ticketJobResult struct {
	AcceptedEvents []domain.TicketEvent
	RejectedEvents []domain.TicketEvent
	RejectedError  string
	DuplicateCount int
	FinalUpdateJob *updateJob
}

type importMetadata struct {
	existingTickets         map[uint]bool
	existingTicketStatuses  map[uint]domain.TicketStatus
	ticketCreatedAt         map[uint]time.Time
	existingDBEvents        map[string]bool
	existingTicketAssignees map[uint]string
}

// buildParsedEvents validates each event and enforces batch size limits.
// This is business logic: the service owns validation rules and size constraints.
func (s *ticketEventService) buildParsedEvents(events []domain.TicketEvent) ([]parsedEvent, error) {
	if len(events) == 0 {
		return nil, errmsgs.ErrEmptyBatch
	}
	if len(events) > maxBatchSize {
		return nil, common.NewBadRequest(
			common.ErrCodeBatchTooLarge,
			fmt.Sprintf("batch size exceeds maximum allowed (limit: %d, got: %d)", maxBatchSize, len(events)),
		)
	}
	parsed := make([]parsedEvent, len(events))
	for i, e := range events {
		parsed[i] = parsedEvent{Event: e, Err: e.Validate()}
	}
	return parsed, nil
}

func (s *ticketEventService) Import(ctx context.Context, events []domain.TicketEvent) (domain.BatchImportResult, error) {
	parsedEvents, err := s.buildParsedEvents(events)
	if err != nil {
		return domain.BatchImportResult{}, err
	}

	workerJobs, rejectedEvents, rejectedCount, ticketIDs, eventKeys := s.filterAndGroupEvents(parsedEvents)

	meta, err := s.fetchMetadata(ctx, ticketIDs, eventKeys)
	if err != nil {
		return domain.BatchImportResult{}, err
	}

	results := worker.Run(workerJobs, func(job ticketWorkerJob) ticketJobResult {
		return s.simulateTicketFSM(job, meta)
	})

	finalResult := domain.BatchImportResult{
		RejectedCount: rejectedCount,
	}

	for errorName, events := range rejectedEvents {
		finalResult.RejectedDetails = append(finalResult.RejectedDetails, domain.RejectedDetail{
			ErrorName: errorName,
			Events:    events,
		})
	}

	err = s.applyImportResults(ctx, results, &finalResult)
	return finalResult, err
}

func (s *ticketEventService) filterAndGroupEvents(parsedEvents []parsedEvent) ([]ticketWorkerJob, map[string][]domain.TicketEvent, int, []uint, []string) {
	validEvents := make([]domain.TicketEvent, 0, len(parsedEvents))
	rejectedEvents := make(map[string][]domain.TicketEvent)
	rejectedCount := 0

	for _, pe := range parsedEvents {
		if pe.Err != nil {
			key := pe.Err.Error()
			rejectedEvents[key] = append(rejectedEvents[key], pe.Event)
			rejectedCount++
			continue
		}
		validEvents = append(validEvents, pe.Event)
	}

	groupedEvents := make(map[uint][]domain.TicketEvent)
	var ticketIDs []uint
	var eventKeys []string

	for _, e := range validEvents {
		if _, ok := groupedEvents[e.TicketID]; !ok {
			ticketIDs = append(ticketIDs, e.TicketID)
		}
		groupedEvents[e.TicketID] = append(groupedEvents[e.TicketID], e)
		eventKeys = append(eventKeys, e.HashKey())
	}

	var workerJobs []ticketWorkerJob
	for id, group := range groupedEvents {
		sort.Slice(group, func(i, j int) bool {
			return group[i].CreatedAt.Before(group[j].CreatedAt)
		})
		workerJobs = append(workerJobs, ticketWorkerJob{TicketID: id, Events: group})
	}

	return workerJobs, rejectedEvents, rejectedCount, ticketIDs, eventKeys
}

func (s *ticketEventService) fetchMetadata(ctx context.Context, ticketIDs []uint, eventKeys []string) (importMetadata, error) {
	existingTickets, err := s.ticketRepo.GetExistingTicketIDs(ctx, ticketIDs)
	if err != nil {
		return importMetadata{}, fmt.Errorf("failed to fetch tickets: %w", err)
	}

	existingTicketStatuses, ticketCreatedAtByTicket, existingTicketAssignees, err := s.ticketRepo.GetTicketStatusAndCreatedAt(ctx, ticketIDs)
	if err != nil {
		return importMetadata{}, fmt.Errorf("failed to fetch ticket metadata: %w", err)
	}

	existingDBEvents, err := s.eventRepo.GetExistingEventKeys(ctx, eventKeys)
	if err != nil {
		return importMetadata{}, fmt.Errorf("failed to fetch existing events: %w", err)
	}

	return importMetadata{
		existingTickets:         existingTickets,
		existingTicketStatuses:  existingTicketStatuses,
		ticketCreatedAt:         ticketCreatedAtByTicket,
		existingDBEvents:        existingDBEvents,
		existingTicketAssignees: existingTicketAssignees,
	}, nil
}

func (s *ticketEventService) simulateTicketFSM(job ticketWorkerJob, meta importMetadata) ticketJobResult {
	var res ticketJobResult
	ticketID := job.TicketID

	if !meta.existingTickets[ticketID] {
		return rejectJob(job, fmt.Errorf("ticket_id %d does not exist in DB", ticketID))
	}

	currentStatus, ok := meta.existingTicketStatuses[ticketID]
	if !ok {
		return rejectJob(job, fmt.Errorf("ticket_id %d does not exist in DB", ticketID))
	}
	ticketCreatedAt := meta.ticketCreatedAt[ticketID]
	currentAssigneeID := meta.existingTicketAssignees[ticketID]

	localSeen := make(map[string]bool)
	var finalJob *updateJob
	var resolvedAt *time.Time
	var cancelledAt *time.Time

	for _, event := range job.Events {
		key := event.HashKey()

		if meta.existingDBEvents[key] || localSeen[key] {
			res.DuplicateCount++
			continue
		}

		if event.FromStatus != currentStatus {
			return rejectJob(job, errmsgs.ErrInvalidFlowTicket)
		}

		if event.ToStatus == domain.StatusResolved || event.ToStatus == domain.StatusCancelled {
			if event.CreatedAt.Before(ticketCreatedAt) {
				status := string(event.ToStatus)
				if len(status) > 0 {
					status = strings.ToUpper(status[:1]) + status[1:]
				}
				return rejectJob(job, fmt.Errorf("%s: %s At cannot be before Created At", errmsgs.ErrInvalidInput.Error(), status))
			}
		}

		reqAssigneeId := strings.TrimSpace(event.AssigneeID)
		if currentStatus == domain.StatusNew && event.ToStatus == domain.StatusAssigned {
			if reqAssigneeId == "" {
				return rejectJob(job, fmt.Errorf("%w: Assignee ID is required when assigning a ticket", errmsgs.ErrInvalidInput))
			}
			currentAssigneeID = reqAssigneeId
		} else if reqAssigneeId != "" && reqAssigneeId != currentAssigneeID {
			return rejectJob(job, fmt.Errorf("%w: Cannot change assignee to '%s' during status transition to '%s'. Current assignee is '%s'",
				errmsgs.ErrInvalidInput, reqAssigneeId, event.ToStatus, currentAssigneeID))
		}

		localSeen[key] = true
		currentStatus = event.ToStatus
		res.AcceptedEvents = append(res.AcceptedEvents, event)

		if event.ToStatus == domain.StatusResolved {
			t := event.CreatedAt
			resolvedAt = &t
		} else if event.ToStatus == domain.StatusCancelled {
			t := event.CreatedAt
			cancelledAt = &t
		}

		finalJob = &updateJob{
			TicketID:    ticketID,
			Status:      event.ToStatus,
			AssigneeID:  currentAssigneeID,
			CreatedAt:   event.CreatedAt,
			ResolvedAt:  resolvedAt,
			CancelledAt: cancelledAt,
		}
	}
	res.FinalUpdateJob = finalJob
	return res
}

func rejectJob(job ticketWorkerJob, err error) ticketJobResult {
	return ticketJobResult{
		RejectedError:  err.Error(),
		RejectedEvents: job.Events,
	}
}

func (s *ticketEventService) applyImportResults(ctx context.Context, results []ticketJobResult, finalResult *domain.BatchImportResult) error {
	var eventsToInsert []domain.TicketEvent
	var finalUpdates []updateJob
	rejectedEvents := make(map[string][]domain.TicketEvent)

	for _, res := range results {
		finalResult.DuplicateCount += res.DuplicateCount

		if res.RejectedError != "" {
			rejectedEvents[res.RejectedError] = append(rejectedEvents[res.RejectedError], res.RejectedEvents...)
			finalResult.RejectedCount += len(res.RejectedEvents)
		}

		if len(res.AcceptedEvents) > 0 {
			eventsToInsert = append(eventsToInsert, res.AcceptedEvents...)
			finalResult.AcceptedCount += len(res.AcceptedEvents)
		}

		if res.FinalUpdateJob != nil {
			finalUpdates = append(finalUpdates, *res.FinalUpdateJob)
		}
	}

	for errorName, events := range rejectedEvents {
		finalResult.RejectedDetails = append(finalResult.RejectedDetails, domain.RejectedDetail{
			ErrorName: errorName,
			Events:    events,
		})
	}

	if len(eventsToInsert) > 0 {
		if err := s.eventRepo.CreateBatch(eventsToInsert); err != nil {
			return err
		}
	}

	if len(finalUpdates) > 0 {
		return s.updateTicketStatuses(ctx, finalUpdates)
	}

	return nil
}

func (s *ticketEventService) updateTicketStatuses(ctx context.Context, finalUpdates []updateJob) error {
	var closedTicketIDs []int
	for _, u := range finalUpdates {
		if u.Status == domain.StatusClosed && u.ResolvedAt == nil {
			closedTicketIDs = append(closedTicketIDs, int(u.TicketID))
		}
	}

	resolvedAtByTicket := make(map[uint]time.Time)
	if len(closedTicketIDs) > 0 {
		resolvedEvents, err := s.eventRepo.FetchLatestResolvedEventPerTicket(ctx, closedTicketIDs)
		if err == nil {
			for _, ev := range resolvedEvents {
				resolvedAtByTicket[ev.TicketID] = ev.CreatedAt
			}
		}
	}

	updateResults := worker.Run(finalUpdates, func(job updateJob) error {
		switch job.Status {
		case domain.StatusResolved:
			if job.ResolvedAt != nil {
				return s.ticketRepo.UpdateStatusAndResolvedAt(ctx, job.TicketID, job.Status, job.AssigneeID, *job.ResolvedAt)
			}
			return s.ticketRepo.UpdateStatusAndResolvedAt(ctx, job.TicketID, job.Status, job.AssigneeID, job.CreatedAt)
		case domain.StatusCancelled:
			if job.CancelledAt != nil {
				return s.ticketRepo.UpdateStatusAndCancelledAt(ctx, job.TicketID, job.Status, job.AssigneeID, *job.CancelledAt)
			}
			return s.ticketRepo.UpdateStatusAndCancelledAt(ctx, job.TicketID, job.Status, job.AssigneeID, job.CreatedAt)
		case domain.StatusClosed:
			if job.ResolvedAt != nil {
				return s.ticketRepo.UpdateStatusAndResolvedAt(ctx, job.TicketID, job.Status, job.AssigneeID, *job.ResolvedAt)
			}
			if resolvedAt, ok := resolvedAtByTicket[job.TicketID]; ok {
				return s.ticketRepo.UpdateStatusAndResolvedAt(ctx, job.TicketID, job.Status, job.AssigneeID, resolvedAt)
			}
			fallthrough
		default:
			return s.ticketRepo.UpdateStatusAndAssignee(ctx, job.TicketID, job.Status, job.AssigneeID)
		}
	})

	for _, err := range updateResults {
		if err != nil {
			return fmt.Errorf("failed to update ticket status: %w", err)
		}
	}
	return nil
}
