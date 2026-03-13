package project

import (
	"context"
)

// AssembleContextPack returns the full context pack for a project (conventions,
// key files, system prompt addendum). Currently just returns the stored pack;
// can later merge in dynamic sources.
func (s *Service) AssembleContextPack(ctx context.Context, projectID string) (*ContextPack, error) {
	p, err := s.store.GetByID(ctx, projectID)
	if err != nil {
		return nil, err
	}
	return &p.ContextPack, nil
}
