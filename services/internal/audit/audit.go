// Package audit records organisation audit events (SECURITY-BASELINE.md §15). Records are
// written in the same tenant transaction as the change they describe, so they exist exactly
// when the change does. Metadata holds changed fields only — never secrets or content.
package audit

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/rizwansammo/MateCollab/services/internal/audit/store"
	"github.com/rizwansammo/MateCollab/services/internal/platform/httpx"
	"github.com/rizwansammo/MateCollab/services/internal/platform/tenant"
)

// Entry is one audited action.
type Entry struct {
	Action     string // e.g. "member.role_changed"
	TargetType string // e.g. "member", "invitation", "organisation"
	TargetID   string
	Outcome    string // success (default), failure, denied
	Metadata   map[string]any
}

// Record writes an entry attributed to a user acting in an organisation.
func Record(ctx context.Context, tx pgx.Tx, orgID tenant.OrgID, actorUserID tenant.UserID, sessionID uuid.UUID,
	meta httpx.ClientMeta, e Entry) error {
	outcome := e.Outcome
	if outcome == "" {
		outcome = "success"
	}
	md := e.Metadata
	if md == nil {
		md = map[string]any{}
	}
	raw, err := json.Marshal(md)
	if err != nil {
		return err
	}
	var actor *uuid.UUID
	actorType := "system"
	if !actorUserID.IsZero() {
		id := uuid.UUID(actorUserID)
		actor, actorType = &id, "user"
	}
	var session *uuid.UUID
	if sessionID != uuid.Nil {
		session = &sessionID
	}
	return store.New(tx).InsertAuditLog(ctx, store.InsertAuditLogParams{
		OrgID:       uuid.UUID(orgID),
		ActorType:   actorType,
		ActorUserID: actor,
		Action:      e.Action,
		TargetType:  ptr(e.TargetType),
		TargetID:    ptr(e.TargetID),
		Outcome:     outcome,
		Ip:          meta.IPPtr(),
		UserAgent:   meta.UserAgentPtr(),
		SessionID:   session,
		RequestID:   meta.RequestIDPtr(),
		Metadata:    raw,
	})
}

// RecordActor is Record for an organisation actor.
func RecordActor(ctx context.Context, tx pgx.Tx, a tenant.Actor, meta httpx.ClientMeta, e Entry) error {
	return Record(ctx, tx, a.OrgID, a.UserID, a.SessionID, meta, e)
}

func ptr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
