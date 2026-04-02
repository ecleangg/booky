package store

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/ecleangg/booky/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (q *Queries) UpsertTaxCase(ctx context.Context, taxCase domain.TaxCase) (uuid.UUID, error) {
	var persistedID uuid.UUID
	err := q.db.QueryRow(ctx, `
		INSERT INTO tax_cases (
			id, bokio_company_id, root_object_type, root_object_id, livemode,
			source_currency, source_amount_minor, sale_type, country, country_source,
			buyer_vat_number, buyer_vat_verified, buyer_is_business, tax_status,
			reportability_state, review_reason, automatic_tax_enabled, automatic_tax_status,
			stripe_tax_amount_known, stripe_tax_amount_minor, stripe_tax_reverse_charge,
			stripe_tax_zero_rated, invoice_pdf_url, dossier
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9, $10,
			$11, $12, $13, $14,
			$15, $16, $17, $18,
			$19, $20, $21,
			$22, $23, $24
		)
		ON CONFLICT (bokio_company_id, root_object_type, root_object_id) DO UPDATE SET
			livemode = EXCLUDED.livemode,
			source_currency = EXCLUDED.source_currency,
			source_amount_minor = EXCLUDED.source_amount_minor,
			sale_type = EXCLUDED.sale_type,
			country = EXCLUDED.country,
			country_source = EXCLUDED.country_source,
			buyer_vat_number = EXCLUDED.buyer_vat_number,
			buyer_vat_verified = EXCLUDED.buyer_vat_verified,
			buyer_is_business = EXCLUDED.buyer_is_business,
			tax_status = EXCLUDED.tax_status,
			reportability_state = EXCLUDED.reportability_state,
			review_reason = EXCLUDED.review_reason,
			automatic_tax_enabled = EXCLUDED.automatic_tax_enabled,
			automatic_tax_status = EXCLUDED.automatic_tax_status,
			stripe_tax_amount_known = EXCLUDED.stripe_tax_amount_known,
			stripe_tax_amount_minor = EXCLUDED.stripe_tax_amount_minor,
			stripe_tax_reverse_charge = EXCLUDED.stripe_tax_reverse_charge,
			stripe_tax_zero_rated = EXCLUDED.stripe_tax_zero_rated,
			invoice_pdf_url = EXCLUDED.invoice_pdf_url,
			dossier = EXCLUDED.dossier,
			updated_at = now()
		RETURNING id
	`, taxCase.ID, taxCase.BokioCompanyID, taxCase.RootObjectType, taxCase.RootObjectID, taxCase.Livemode,
		taxCase.SourceCurrency, taxCase.SourceAmountMinor, taxCase.SaleType, taxCase.Country, taxCase.CountrySource,
		taxCase.BuyerVATNumber, taxCase.BuyerVATVerified, taxCase.BuyerIsBusiness, taxCase.TaxStatus,
		taxCase.ReportabilityState, taxCase.ReviewReason, taxCase.AutomaticTaxEnabled, taxCase.AutomaticTaxStatus,
		taxCase.StripeTaxAmountKnown, taxCase.StripeTaxAmountMinor, taxCase.StripeTaxReverseCharge,
		taxCase.StripeTaxZeroRated, taxCase.InvoicePDFURL, []byte(taxCase.Dossier)).Scan(&persistedID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("upsert tax case %s:%s: %w", taxCase.RootObjectType, taxCase.RootObjectID, err)
	}
	return persistedID, nil
}

func (q *Queries) GetTaxCase(ctx context.Context, id uuid.UUID) (domain.TaxCase, error) {
	row := q.db.QueryRow(ctx, `
		SELECT id, bokio_company_id, root_object_type, root_object_id, livemode,
			source_currency, source_amount_minor, sale_type, country, country_source,
			buyer_vat_number, buyer_vat_verified, buyer_is_business, tax_status,
			reportability_state, review_reason, automatic_tax_enabled, automatic_tax_status,
			stripe_tax_amount_known, stripe_tax_amount_minor, stripe_tax_reverse_charge,
			stripe_tax_zero_rated, invoice_pdf_url, dossier, created_at, updated_at
		FROM tax_cases
		WHERE id = $1
	`, id)
	return scanTaxCase(row)
}

func (q *Queries) GetTaxCaseByRoot(ctx context.Context, companyID uuid.UUID, rootType, rootID string) (domain.TaxCase, error) {
	row := q.db.QueryRow(ctx, `
		SELECT id, bokio_company_id, root_object_type, root_object_id, livemode,
			source_currency, source_amount_minor, sale_type, country, country_source,
			buyer_vat_number, buyer_vat_verified, buyer_is_business, tax_status,
			reportability_state, review_reason, automatic_tax_enabled, automatic_tax_status,
			stripe_tax_amount_known, stripe_tax_amount_minor, stripe_tax_reverse_charge,
			stripe_tax_zero_rated, invoice_pdf_url, dossier, created_at, updated_at
		FROM tax_cases
		WHERE bokio_company_id = $1 AND root_object_type = $2 AND root_object_id = $3
	`, companyID, rootType, rootID)
	return scanTaxCase(row)
}

func (q *Queries) ListTaxCasesByIDs(ctx context.Context, ids []uuid.UUID) ([]domain.TaxCase, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	rows, err := q.db.Query(ctx, `
		SELECT id, bokio_company_id, root_object_type, root_object_id, livemode,
			source_currency, source_amount_minor, sale_type, country, country_source,
			buyer_vat_number, buyer_vat_verified, buyer_is_business, tax_status,
			reportability_state, review_reason, automatic_tax_enabled, automatic_tax_status,
			stripe_tax_amount_known, stripe_tax_amount_minor, stripe_tax_reverse_charge,
			stripe_tax_zero_rated, invoice_pdf_url, dossier, created_at, updated_at
		FROM tax_cases
		WHERE id = ANY($1)
		ORDER BY root_object_type, root_object_id
	`, ids)
	if err != nil {
		return nil, fmt.Errorf("list tax cases by ids: %w", err)
	}
	defer rows.Close()

	var out []domain.TaxCase
	for rows.Next() {
		entry, err := scanTaxCase(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate tax cases: %w", err)
	}
	return out, nil
}

func (q *Queries) ListTaxCasesByObject(ctx context.Context, objectType, objectID string) ([]domain.TaxCase, error) {
	rows, err := q.db.Query(ctx, `
		SELECT c.id, c.bokio_company_id, c.root_object_type, c.root_object_id, c.livemode,
			c.source_currency, c.source_amount_minor, c.sale_type, c.country, c.country_source,
			c.buyer_vat_number, c.buyer_vat_verified, c.buyer_is_business, c.tax_status,
			c.reportability_state, c.review_reason, c.automatic_tax_enabled, c.automatic_tax_status,
			c.stripe_tax_amount_known, c.stripe_tax_amount_minor, c.stripe_tax_reverse_charge,
			c.stripe_tax_zero_rated, c.invoice_pdf_url, c.dossier, c.created_at, c.updated_at
		FROM tax_cases c
		JOIN tax_case_objects o ON o.tax_case_id = c.id
		WHERE o.object_type = $1 AND o.object_id = $2
		ORDER BY c.root_object_type, c.root_object_id
	`, objectType, objectID)
	if err != nil {
		return nil, fmt.Errorf("list tax cases by object: %w", err)
	}
	defer rows.Close()

	var out []domain.TaxCase
	for rows.Next() {
		entry, err := scanTaxCase(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate tax cases by object: %w", err)
	}
	return out, nil
}

func (q *Queries) ReplaceTaxCaseObjects(ctx context.Context, taxCaseID uuid.UUID, objects []domain.TaxCaseObject) error {
	if _, err := q.db.Exec(ctx, `DELETE FROM tax_case_objects WHERE tax_case_id = $1`, taxCaseID); err != nil {
		return fmt.Errorf("delete tax case objects %s: %w", taxCaseID, err)
	}
	for _, object := range objects {
		_, err := q.db.Exec(ctx, `
			INSERT INTO tax_case_objects (tax_case_id, object_type, object_id, object_role)
			VALUES ($1, $2, $3, $4)
		`, taxCaseID, object.ObjectType, object.ObjectID, object.ObjectRole)
		if err != nil {
			return fmt.Errorf("insert tax case object %s:%s: %w", object.ObjectType, object.ObjectID, err)
		}
	}
	return nil
}

func (q *Queries) ListTaxCaseObjects(ctx context.Context, taxCaseID uuid.UUID) ([]domain.TaxCaseObject, error) {
	rows, err := q.db.Query(ctx, `
		SELECT tax_case_id, object_type, object_id, object_role, created_at
		FROM tax_case_objects
		WHERE tax_case_id = $1
		ORDER BY object_role, object_type, object_id
	`, taxCaseID)
	if err != nil {
		return nil, fmt.Errorf("list tax case objects: %w", err)
	}
	defer rows.Close()

	var out []domain.TaxCaseObject
	for rows.Next() {
		var entry domain.TaxCaseObject
		if err := rows.Scan(&entry.TaxCaseID, &entry.ObjectType, &entry.ObjectID, &entry.ObjectRole, &entry.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan tax case object: %w", err)
		}
		out = append(out, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate tax case objects: %w", err)
	}
	return out, nil
}

func (q *Queries) UpsertManualTaxEvidence(ctx context.Context, evidence domain.ManualTaxEvidence) error {
	_, err := q.db.Exec(ctx, `
		INSERT INTO manual_tax_evidence (
			id, tax_case_id, country, country_source, buyer_vat_number,
			buyer_vat_verified, buyer_is_business, sale_type, note, payload
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9, $10
		)
		ON CONFLICT (id) DO UPDATE SET
			country = EXCLUDED.country,
			country_source = EXCLUDED.country_source,
			buyer_vat_number = EXCLUDED.buyer_vat_number,
			buyer_vat_verified = EXCLUDED.buyer_vat_verified,
			buyer_is_business = EXCLUDED.buyer_is_business,
			sale_type = EXCLUDED.sale_type,
			note = EXCLUDED.note,
			payload = EXCLUDED.payload,
			updated_at = now()
	`, evidence.ID, evidence.TaxCaseID, evidence.Country, evidence.CountrySource, evidence.BuyerVATNumber,
		evidence.BuyerVATVerified, evidence.BuyerIsBusiness, evidence.SaleType, evidence.Note, []byte(evidence.Payload))
	if err != nil {
		return fmt.Errorf("upsert manual tax evidence: %w", err)
	}
	return nil
}

func (q *Queries) ListManualTaxEvidence(ctx context.Context, taxCaseID uuid.UUID) ([]domain.ManualTaxEvidence, error) {
	rows, err := q.db.Query(ctx, `
		SELECT id, tax_case_id, country, country_source, buyer_vat_number,
			buyer_vat_verified, buyer_is_business, sale_type, note, payload,
			created_at, updated_at
		FROM manual_tax_evidence
		WHERE tax_case_id = $1
		ORDER BY created_at
	`, taxCaseID)
	if err != nil {
		return nil, fmt.Errorf("list manual tax evidence: %w", err)
	}
	defer rows.Close()

	var out []domain.ManualTaxEvidence
	for rows.Next() {
		var entry domain.ManualTaxEvidence
		var country, countrySource, buyerVATNumber, saleType, note sql.NullString
		var buyerVATVerified, buyerIsBusiness sql.NullBool
		var payload []byte
		if err := rows.Scan(&entry.ID, &entry.TaxCaseID, &country, &countrySource, &buyerVATNumber,
			&buyerVATVerified, &buyerIsBusiness, &saleType, &note, &payload, &entry.CreatedAt, &entry.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan manual tax evidence: %w", err)
		}
		if country.Valid {
			entry.Country = &country.String
		}
		if countrySource.Valid {
			entry.CountrySource = &countrySource.String
		}
		if buyerVATNumber.Valid {
			entry.BuyerVATNumber = &buyerVATNumber.String
		}
		if buyerVATVerified.Valid {
			entry.BuyerVATVerified = &buyerVATVerified.Bool
		}
		if buyerIsBusiness.Valid {
			entry.BuyerIsBusiness = &buyerIsBusiness.Bool
		}
		if saleType.Valid {
			entry.SaleType = &saleType.String
		}
		if note.Valid {
			entry.Note = &note.String
		}
		entry.Payload = payload
		out = append(out, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate manual tax evidence: %w", err)
	}
	return out, nil
}

func scanTaxCase(row interface{ Scan(dest ...any) error }) (domain.TaxCase, error) {
	var entry domain.TaxCase
	var sourceCurrency, saleType, country, countrySource, buyerVATNumber sql.NullString
	var taxStatus, reviewReason, automaticTaxStatus, invoicePDFURL sql.NullString
	var sourceAmountMinor, stripeTaxAmountMinor sql.NullInt64
	var dossier []byte
	if err := row.Scan(
		&entry.ID,
		&entry.BokioCompanyID,
		&entry.RootObjectType,
		&entry.RootObjectID,
		&entry.Livemode,
		&sourceCurrency,
		&sourceAmountMinor,
		&saleType,
		&country,
		&countrySource,
		&buyerVATNumber,
		&entry.BuyerVATVerified,
		&entry.BuyerIsBusiness,
		&taxStatus,
		&entry.ReportabilityState,
		&reviewReason,
		&entry.AutomaticTaxEnabled,
		&automaticTaxStatus,
		&entry.StripeTaxAmountKnown,
		&stripeTaxAmountMinor,
		&entry.StripeTaxReverseCharge,
		&entry.StripeTaxZeroRated,
		&invoicePDFURL,
		&dossier,
		&entry.CreatedAt,
		&entry.UpdatedAt,
	); err != nil {
		if err == pgx.ErrNoRows {
			return domain.TaxCase{}, ErrNotFound
		}
		return domain.TaxCase{}, fmt.Errorf("scan tax case: %w", err)
	}
	if sourceCurrency.Valid {
		entry.SourceCurrency = &sourceCurrency.String
	}
	if sourceAmountMinor.Valid {
		entry.SourceAmountMinor = &sourceAmountMinor.Int64
	}
	if saleType.Valid {
		entry.SaleType = &saleType.String
	}
	if country.Valid {
		entry.Country = &country.String
	}
	if countrySource.Valid {
		entry.CountrySource = &countrySource.String
	}
	if buyerVATNumber.Valid {
		entry.BuyerVATNumber = &buyerVATNumber.String
	}
	if taxStatus.Valid {
		entry.TaxStatus = &taxStatus.String
	}
	if reviewReason.Valid {
		entry.ReviewReason = &reviewReason.String
	}
	if automaticTaxStatus.Valid {
		entry.AutomaticTaxStatus = &automaticTaxStatus.String
	}
	if stripeTaxAmountMinor.Valid {
		entry.StripeTaxAmountMinor = &stripeTaxAmountMinor.Int64
	}
	if invoicePDFURL.Valid {
		entry.InvoicePDFURL = &invoicePDFURL.String
	}
	entry.Dossier = dossier
	return entry, nil
}
