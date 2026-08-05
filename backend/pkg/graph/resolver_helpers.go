package graph

import (
	"context"
	"time"

	"pentagi/pkg/database"
	"pentagi/pkg/database/converter"
	"pentagi/pkg/graph/model"

	"github.com/sirupsen/logrus"
)

// These are hand-written resolver helpers. They live outside *.resolvers.go so
// gqlgen's follow-schema layout does not relocate them into its "WARNING" block
// on the next regeneration.

// createAssistantProvisionTimeout bounds background assistant provisioning so a
// stalled LLM/executor cold start cannot leak the goroutine (and the flow
// controller lock it holds) indefinitely.
const createAssistantProvisionTimeout = 5 * time.Minute

// flowAssistantResponse builds the CreateAssistant mutation payload from the
// current DB state for the given assistant.
func (r *mutationResolver) flowAssistantResponse(ctx context.Context, assistantID int64) (*model.FlowAssistant, error) {
	assistant, err := r.DB.GetAssistant(ctx, assistantID)
	if err != nil {
		return nil, err
	}

	flow, err := r.DB.GetFlow(ctx, assistant.FlowID)
	if err != nil {
		return nil, err
	}

	containers, err := r.DB.GetFlowContainers(ctx, assistant.FlowID)
	if err != nil {
		return nil, err
	}

	return converter.ConvertFlowAssistant(flow, containers, assistant), nil
}

// stopAssistantFallback handles StopAssistant when no in-memory worker exists.
// If the assistant row is present it is returned as-is (no error), so the UI
// can recover; otherwise the original lookup error is surfaced.
func (r *mutationResolver) stopAssistantFallback(
	ctx context.Context, flowID, assistantID int64, cause error,
) (*model.Assistant, error) {
	assistant, dbErr := r.DB.GetFlowAssistant(ctx, database.GetFlowAssistantParams{
		ID:     assistantID,
		FlowID: flowID,
	})
	if dbErr != nil {
		return nil, cause
	}

	r.Logger.WithFields(logrus.Fields{
		"flow":      flowID,
		"assistant": assistantID,
	}).WithError(cause).Warn("stop assistant: no in-memory worker; returning current state")

	return converter.ConvertAssistant(assistant), nil
}
