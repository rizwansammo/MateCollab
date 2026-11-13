package collaboration

import (
	"github.com/google/uuid"
	"github.com/rizwansammo/MateCollab/services/internal/platform/tenant"
	"time"
)

// Event is the mc.v1 WebSocket envelope for future post-commit realtime delivery.
// Phase 3 does not publish these events: Phase 4 implements the transport.
type Event struct {
	Version   int            `json:"v"`
	ID        uuid.UUID      `json:"id"`
	Type      string         `json:"type"`
	Timestamp time.Time      `json:"ts"`
	OrgID     uuid.UUID      `json:"org_id"`
	Data      map[string]any `json:"data"`
}

// NewEvent allocates the envelope for a Space/Channel lifecycle event.
// The caller must emit it only after the transaction is committed.
func NewEvent(org tenant.OrgID, typ string, resource uuid.UUID, at time.Time) (Event, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return Event{}, err
	}
	return Event{Version: 1, ID: id, Type: typ, Timestamp: at.UTC(), OrgID: uuid.UUID(org),
		Data: map[string]any{"id": resource.String()}}, nil
}
