package stripe

import (
	"github.com/ecleangg/booky/internal/filings"
	"github.com/ecleangg/booky/internal/integrations"
	"github.com/ecleangg/booky/internal/tax"
)

func (s *Service) withRuntime(runtime integrations.RuntimeConfig) *Service {
	cfg := runtime.Apply(s.Config)
	copy := *s
	copy.Config = cfg
	copy.Client = s.Client.WithAccount(runtime.StripeAccountID)
	copy.Tax = tax.NewService(cfg, s.Repo, s.Logger)
	copy.Filings = filings.NewService(cfg, s.Repo, s.Notify, s.Tenants, s.Logger)
	return &copy
}
