package store

import (
	"context"
	"fmt"

	"github.com/ecleangg/booky/internal/domain"
	"github.com/google/uuid"
)

func (q *Queries) CreateFilingExport(ctx context.Context, export domain.FilingExport) error {
	_, err := q.db.Exec(ctx, `
		INSERT INTO filing_exports (
			id, kind, period, bokio_company_id, version, checksum, filename, content,
			summary, emailed_at, superseded_by
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11
		)
	`, export.ID, export.Kind, export.Period, export.BokioCompanyID, export.Version, export.Checksum, export.Filename, export.Content, nullableJSON(export.Summary), export.EmailedAt, export.SupersededBy)
	if err != nil {
		return fmt.Errorf("create filing export: %w", err)
	}
	return nil
}

func (q *Queries) GetLatestFilingExport(ctx context.Context, companyID uuid.UUID, kind, period string) (domain.FilingExport, error) {
	row := q.db.QueryRow(ctx, `
		SELECT id, kind, period, bokio_company_id, version, checksum, filename, content,
			summary, emailed_at, superseded_by, created_at, updated_at
		FROM filing_exports
		WHERE bokio_company_id = $1 AND kind = $2 AND period = $3
		ORDER BY version DESC
		LIMIT 1
	`, companyID, kind, period)

	return scanFilingExport(row)
}

func (q *Queries) ListFilingExports(ctx context.Context, companyID uuid.UUID, kind, period string) ([]domain.FilingExport, error) {
	rows, err := q.db.Query(ctx, `
		SELECT id, kind, period, bokio_company_id, version, checksum, filename, content,
			summary, emailed_at, superseded_by, created_at, updated_at
		FROM filing_exports
		WHERE bokio_company_id = $1 AND kind = $2 AND period = $3
		ORDER BY version DESC
	`, companyID, kind, period)
	if err != nil {
		return nil, fmt.Errorf("list filing exports: %w", err)
	}
	defer rows.Close()

	var exports []domain.FilingExport
	for rows.Next() {
		export, err := scanFilingExport(rows)
		if err != nil {
			return nil, err
		}
		exports = append(exports, export)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate filing exports: %w", err)
	}
	return exports, nil
}

func (q *Queries) MarkFilingExportSuperseded(ctx context.Context, exportID, supersededBy uuid.UUID) error {
	_, err := q.db.Exec(ctx, `
		UPDATE filing_exports
		SET superseded_by = $2, updated_at = now()
		WHERE id = $1
	`, exportID, supersededBy)
	if err != nil {
		return fmt.Errorf("mark filing export superseded: %w", err)
	}
	return nil
}
