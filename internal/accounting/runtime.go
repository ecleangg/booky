package accounting

import (
	"context"
	"time"

	"github.com/ecleangg/booky/internal/bokio"
	"github.com/ecleangg/booky/internal/integrations"
	"github.com/google/uuid"
)

func (s *Service) withRuntime(runtime integrations.RuntimeConfig) *Service {
	cfg := runtime.Apply(s.Config)
	copy := *s
	copy.Config = cfg
	copy.Bokio = bokio.NewClient(cfg.Bokio)
	return &copy
}

func (s *Service) RunDailyCloseForCompany(ctx context.Context, companyID uuid.UUID, postingDate time.Time) error {
	if s.Tenants == nil {
		return s.RunDailyClose(ctx, postingDate)
	}
	runtime, err := s.Tenants.ResolveRuntimeByCompanyID(ctx, companyID)
	if err != nil {
		return err
	}
	return s.withRuntime(runtime).RunDailyClose(ctx, postingDate)
}
