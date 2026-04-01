package support

import (
	"time"

	"github.com/ecleangg/booky/internal/config"
)

func LocationOrUTC(cfg config.Config) *time.Location {
	loc, err := cfg.Location()
	if err != nil {
		return time.UTC
	}
	return loc
}
