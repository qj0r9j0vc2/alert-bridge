package slack

import (
	"context"
	"fmt"
	"time"

	slackLib "github.com/slack-go/slack"

	"github.com/qj0r9j0vc2/alert-bridge/internal/adapter/dto"
	"github.com/qj0r9j0vc2/alert-bridge/internal/domain/entity"
	"github.com/qj0r9j0vc2/alert-bridge/internal/domain/repository"
	slackInfra "github.com/qj0r9j0vc2/alert-bridge/internal/infrastructure/slack"
)

// SilenceResult represents the result of a silence operation.
type SilenceResult struct {
	Action      dto.SilenceAction
	Silences    []*entity.SilenceMark
	Created     *entity.SilenceMark
	Deleted     *entity.SilenceMark
	Message     string
	OpenedModal bool // True if a modal was opened (no message response needed)
}

// SilenceModalClient defines the Slack client operations needed for modal handling.
type SilenceModalClient interface {
	OpenModal(ctx context.Context, triggerID string, view slackLib.ModalViewRequest) error
	GetActiveAlertLabels(ctx context.Context, alertRepo interface {
		GetActiveAlerts(ctx context.Context, severity string) ([]*entity.Alert, error)
	}) (map[string][]string, error)
}

// ManageSilenceUseCase handles silence management via slash commands.
type ManageSilenceUseCase struct {
	silenceRepo repository.SilenceRepository
	alertRepo   repository.AlertRepository
	slackClient SilenceModalClient
}

// NewManageSilenceUseCase creates a new manage silence use case.
func NewManageSilenceUseCase(
	silenceRepo repository.SilenceRepository,
	alertRepo repository.AlertRepository,
	slackClient SilenceModalClient,
) *ManageSilenceUseCase {
	return &ManageSilenceUseCase{
		silenceRepo: silenceRepo,
		alertRepo:   alertRepo,
		slackClient: slackClient,
	}
}

// Execute performs the requested silence action.
func (uc *ManageSilenceUseCase) Execute(ctx context.Context, req *dto.SilenceRequest) (*SilenceResult, error) {
	switch req.Action {
	case dto.SilenceActionOpenModal:
		return uc.openModal(ctx, req)
	case dto.SilenceActionCreate, dto.SilenceActionFromModal:
		return uc.createSilence(ctx, req)
	case dto.SilenceActionList:
		return uc.listSilences(ctx)
	case dto.SilenceActionDelete:
		return uc.deleteSilence(ctx, req)
	default:
		return nil, fmt.Errorf("unknown action: %s", req.Action)
	}
}

// openModal opens the silence creation modal.
func (uc *ManageSilenceUseCase) openModal(ctx context.Context, req *dto.SilenceRequest) (*SilenceResult, error) {
	if req.TriggerID == "" {
		return nil, fmt.Errorf("trigger ID is required to open modal")
	}

	// Get active alert labels for autocomplete
	var labelOptions map[string][]string
	if uc.slackClient != nil && uc.alertRepo != nil {
		var err error
		labelOptions, err = uc.slackClient.GetActiveAlertLabels(ctx, uc.alertRepo)
		if err != nil {
			// Log but continue without label options
			labelOptions = nil
		}
	}

	// Build and open the modal
	modal := slackInfra.BuildSilenceModal(labelOptions)
	if err := uc.slackClient.OpenModal(ctx, req.TriggerID, modal); err != nil {
		return nil, fmt.Errorf("failed to open modal: %w", err)
	}

	return &SilenceResult{
		Action:      dto.SilenceActionOpenModal,
		Message:     "Opening silence creation form...",
		OpenedModal: true,
	}, nil
}

// createSilence creates a new silence.
func (uc *ManageSilenceUseCase) createSilence(ctx context.Context, req *dto.SilenceRequest) (*SilenceResult, error) {
	if req.Duration <= 0 {
		req.Duration = 1 * time.Hour // Default duration
	}

	silence, err := entity.NewSilenceMark(
		req.Duration,
		req.UserName,
		"", // email not available from Slack user ID
		entity.AckSourceSlack,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create silence: %w", err)
	}

	if req.Reason != "" {
		silence.WithReason(req.Reason)
	}

	// Add label matchers if provided
	if len(req.Matchers) > 0 {
		silence.WithMatchers(req.Matchers)
	}

	if err := uc.silenceRepo.Save(ctx, silence); err != nil {
		return nil, fmt.Errorf("failed to save silence: %w", err)
	}

	// Build message with matcher info
	msg := fmt.Sprintf("Created silence for %s", formatDuration(req.Duration))
	if len(req.Matchers) > 0 {
		msg += fmt.Sprintf(" with %d matcher(s)", len(req.Matchers))
	}

	return &SilenceResult{
		Action:  dto.SilenceActionCreate,
		Created: silence,
		Message: msg,
	}, nil
}

// listSilences returns all active silences.
func (uc *ManageSilenceUseCase) listSilences(ctx context.Context) (*SilenceResult, error) {
	silences, err := uc.silenceRepo.FindActive(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list silences: %w", err)
	}

	return &SilenceResult{
		Action:   dto.SilenceActionList,
		Silences: silences,
		Message:  fmt.Sprintf("Found %d active silence(s)", len(silences)),
	}, nil
}

// deleteSilence removes a silence by ID.
func (uc *ManageSilenceUseCase) deleteSilence(ctx context.Context, req *dto.SilenceRequest) (*SilenceResult, error) {
	if req.SilenceID == "" {
		return nil, fmt.Errorf("silence ID is required for delete action")
	}

	// Find the silence first to return it in the result
	silence, err := uc.silenceRepo.FindByID(ctx, req.SilenceID)
	if err != nil {
		return nil, fmt.Errorf("failed to find silence: %w", err)
	}
	if silence == nil {
		return nil, fmt.Errorf("silence not found: %s", req.SilenceID)
	}

	if err := uc.silenceRepo.Delete(ctx, req.SilenceID); err != nil {
		return nil, fmt.Errorf("failed to delete silence: %w", err)
	}

	return &SilenceResult{
		Action:  dto.SilenceActionDelete,
		Deleted: silence,
		Message: fmt.Sprintf("Deleted silence %s", req.SilenceID),
	}, nil
}
