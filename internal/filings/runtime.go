package filings

import (
	"context"
	"time"

	"github.com/ecleangg/booky/internal/integrations"
	"github.com/google/uuid"
)

func (s *Service) withRuntime(runtime integrations.RuntimeConfig) *Service {
	copy := *s
	copy.Config = runtime.Apply(s.Config)
	return &copy
}

func (s *Service) EvaluateDuePeriodsForCompany(ctx context.Context, companyID uuid.UUID, now time.Time) error {
	if s.Tenants == nil {
		return s.EvaluateDuePeriods(ctx, now)
	}
	runtime, err := s.Tenants.ResolveRuntimeByCompanyID(ctx, companyID)
	if err != nil {
		return err
	}
	return s.withRuntime(runtime).EvaluateDuePeriods(ctx, now)
}
