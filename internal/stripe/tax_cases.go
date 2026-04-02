package stripe

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/ecleangg/booky/internal/domain"
	"github.com/ecleangg/booky/internal/store"
)

func (s *Service) buildTaxCase(ctx context.Context, livemode bool, snapshots []domain.ObjectSnapshot) (domain.TaxCase, []domain.TaxCaseObject, error) {
	if s.Tax == nil {
		return domain.TaxCase{}, nil, fmt.Errorf("tax service is not configured")
	}
	return s.Tax.BuildCaseFromSnapshots(ctx, livemode, dedupeSnapshots(snapshots))
}

func dedupeSnapshots(snapshots []domain.ObjectSnapshot) []domain.ObjectSnapshot {
	seen := map[string]struct{}{}
	out := make([]domain.ObjectSnapshot, 0, len(snapshots))
	for _, snapshot := range snapshots {
		key := snapshot.ObjectType + ":" + snapshot.ObjectID
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, snapshot)
	}
	return out
}

func (s *Service) saleSnapshotsForCharge(ctx context.Context, evt Event, charge Charge, existing ...domain.ObjectSnapshot) ([]domain.ObjectSnapshot, error) {
	snapshots := append([]domain.ObjectSnapshot{}, existing...)
	paymentIntentID := strings.TrimSpace(charge.PaymentIntent)
	if paymentIntentID != "" {
		pi, raw, err := s.Client.GetPaymentIntent(ctx, paymentIntentID)
		if err != nil {
			return nil, err
		}
		snapshots = append(snapshots, domain.ObjectSnapshot{ObjectType: "payment_intent", ObjectID: pi.ID, Livemode: evt.Livemode, Payload: raw})
		if paymentIntentID == "" {
			paymentIntentID = pi.ID
		}
	}
	if charge.Invoice != "" {
		invoice, raw, err := s.Client.GetInvoice(ctx, charge.Invoice)
		if err != nil {
			return nil, err
		}
		snapshots = append(snapshots, domain.ObjectSnapshot{ObjectType: "invoice", ObjectID: invoice.ID, Livemode: evt.Livemode, Payload: raw})
	}
	if paymentIntentID != "" {
		sessions, raws, err := s.Client.ListCheckoutSessionsByPaymentIntent(ctx, paymentIntentID)
		if err != nil {
			return nil, err
		}
		for i, session := range sessions {
			snapshots = append(snapshots, domain.ObjectSnapshot{ObjectType: "checkout_session", ObjectID: session.ID, Livemode: evt.Livemode, Payload: raws[i]})
			if charge.Invoice == "" && strings.TrimSpace(session.Invoice) != "" {
				invoice, raw, err := s.Client.GetInvoice(ctx, session.Invoice)
				if err != nil {
					return nil, err
				}
				snapshots = append(snapshots, domain.ObjectSnapshot{ObjectType: "invoice", ObjectID: invoice.ID, Livemode: evt.Livemode, Payload: raw})
			}
		}
	}

	customerID := strings.TrimSpace(charge.Customer)
	for _, snapshot := range snapshots {
		if snapshot.ObjectType != "invoice" || customerID != "" {
			continue
		}
		var invoice Invoice
		if err := json.Unmarshal(snapshot.Payload, &invoice); err == nil {
			customerID = strings.TrimSpace(invoice.Customer)
		}
	}
	if customerID != "" {
		customer, customerRaw, err := s.Client.GetCustomer(ctx, customerID)
		if err != nil {
			return nil, err
		}
		snapshots = append(snapshots, domain.ObjectSnapshot{ObjectType: "customer", ObjectID: customer.ID, Livemode: evt.Livemode, Payload: customerRaw})
		taxIDs, raws, err := s.Client.ListCustomerTaxIDs(ctx, customerID)
		if err != nil {
			return nil, err
		}
		for i, taxID := range taxIDs {
			snapshots = append(snapshots, domain.ObjectSnapshot{ObjectType: "tax_id", ObjectID: taxID.ID, Livemode: evt.Livemode, Payload: raws[i]})
		}
	}
	return dedupeSnapshots(snapshots), nil
}

func (s *Service) saleSnapshotsForCheckoutSession(ctx context.Context, evt Event, session CheckoutSession) ([]domain.ObjectSnapshot, error) {
	snapshots := []domain.ObjectSnapshot{}
	if session.Invoice != "" {
		invoice, raw, err := s.Client.GetInvoice(ctx, session.Invoice)
		if err != nil {
			return nil, err
		}
		snapshots = append(snapshots, domain.ObjectSnapshot{ObjectType: "invoice", ObjectID: invoice.ID, Livemode: evt.Livemode, Payload: raw})
	}
	if session.PaymentIntent != "" {
		pi, raw, err := s.Client.GetPaymentIntent(ctx, session.PaymentIntent)
		if err != nil {
			return nil, err
		}
		snapshots = append(snapshots, domain.ObjectSnapshot{ObjectType: "payment_intent", ObjectID: pi.ID, Livemode: evt.Livemode, Payload: raw})
		if pi.LatestCharge.ID != "" {
			charge, chargeRaw, err := s.getChargeReadyForAccounting(ctx, pi.LatestCharge.ID)
			if err != nil {
				return nil, err
			}
			snapshots = append(snapshots, domain.ObjectSnapshot{ObjectType: "charge", ObjectID: charge.ID, Livemode: evt.Livemode, Payload: chargeRaw})
			more, err := s.saleSnapshotsForCharge(ctx, evt, charge, snapshots...)
			if err != nil {
				return nil, err
			}
			return more, nil
		}
	}
	if session.Customer != "" {
		customer, raw, err := s.Client.GetCustomer(ctx, session.Customer)
		if err != nil {
			return nil, err
		}
		snapshots = append(snapshots, domain.ObjectSnapshot{ObjectType: "customer", ObjectID: customer.ID, Livemode: evt.Livemode, Payload: raw})
		taxIDs, raws, err := s.Client.ListCustomerTaxIDs(ctx, session.Customer)
		if err != nil {
			return nil, err
		}
		for i, item := range taxIDs {
			snapshots = append(snapshots, domain.ObjectSnapshot{ObjectType: "tax_id", ObjectID: item.ID, Livemode: evt.Livemode, Payload: raws[i]})
		}
	}
	return dedupeSnapshots(snapshots), nil
}

func (s *Service) rebuildStoredTaxCase(ctx context.Context, taxCase domain.TaxCase) (domain.TaxCase, []domain.TaxCaseObject, []domain.ObjectSnapshot, error) {
	objects, err := s.Repo.Queries().ListTaxCaseObjects(ctx, taxCase.ID)
	if err != nil {
		return domain.TaxCase{}, nil, nil, err
	}
	snapshots := make([]domain.ObjectSnapshot, 0, len(objects))
	customerIDs := map[string]struct{}{}
	for _, object := range objects {
		snapshot, fresh, err := s.refreshSnapshot(ctx, taxCase.Livemode, object)
		if err != nil {
			return domain.TaxCase{}, nil, nil, err
		}
		if !fresh {
			snapshot, err = s.Repo.Queries().GetObjectSnapshot(ctx, object.ObjectType, object.ObjectID)
			if err != nil {
				if errors.Is(err, store.ErrNotFound) {
					continue
				}
				return domain.TaxCase{}, nil, nil, err
			}
		}
		if snapshot.ObjectType == "customer" {
			customerIDs[snapshot.ObjectID] = struct{}{}
		}
		snapshots = append(snapshots, snapshot)
	}
	for customerID := range customerIDs {
		taxIDs, raws, err := s.Client.ListCustomerTaxIDs(ctx, customerID)
		if err != nil {
			return domain.TaxCase{}, nil, nil, err
		}
		for i, taxID := range taxIDs {
			snapshots = append(snapshots, domain.ObjectSnapshot{ObjectType: "tax_id", ObjectID: taxID.ID, Livemode: taxCase.Livemode, Payload: raws[i]})
		}
	}
	if len(snapshots) == 0 {
		return domain.TaxCase{}, nil, nil, fmt.Errorf("no stored snapshots for tax case %s", taxCase.ID)
	}
	rebuilt, caseObjects, err := s.Tax.BuildCaseFromSnapshots(ctx, taxCase.Livemode, snapshots)
	if err != nil {
		return domain.TaxCase{}, nil, nil, err
	}
	rebuilt.ID = taxCase.ID
	for i := range caseObjects {
		caseObjects[i].TaxCaseID = taxCase.ID
	}
	return rebuilt, caseObjects, snapshots, nil
}

func (s *Service) refreshSnapshot(ctx context.Context, livemode bool, object domain.TaxCaseObject) (domain.ObjectSnapshot, bool, error) {
	switch object.ObjectType {
	case "checkout_session":
		session, raw, err := s.Client.GetCheckoutSession(ctx, object.ObjectID)
		if err != nil {
			return domain.ObjectSnapshot{}, false, nil
		}
		return domain.ObjectSnapshot{ObjectType: "checkout_session", ObjectID: session.ID, Livemode: livemode, Payload: raw}, true, nil
	case "invoice":
		invoice, raw, err := s.Client.GetInvoice(ctx, object.ObjectID)
		if err != nil {
			return domain.ObjectSnapshot{}, false, nil
		}
		return domain.ObjectSnapshot{ObjectType: "invoice", ObjectID: invoice.ID, Livemode: livemode, Payload: raw}, true, nil
	case "payment_intent":
		pi, raw, err := s.Client.GetPaymentIntent(ctx, object.ObjectID)
		if err != nil {
			return domain.ObjectSnapshot{}, false, nil
		}
		return domain.ObjectSnapshot{ObjectType: "payment_intent", ObjectID: pi.ID, Livemode: livemode, Payload: raw}, true, nil
	case "charge":
		charge, raw, err := s.Client.GetCharge(ctx, object.ObjectID)
		if err != nil {
			return domain.ObjectSnapshot{}, false, nil
		}
		return domain.ObjectSnapshot{ObjectType: "charge", ObjectID: charge.ID, Livemode: livemode, Payload: raw}, true, nil
	case "refund":
		refund, raw, err := s.Client.GetRefund(ctx, object.ObjectID)
		if err != nil {
			return domain.ObjectSnapshot{}, false, nil
		}
		return domain.ObjectSnapshot{ObjectType: "refund", ObjectID: refund.ID, Livemode: livemode, Payload: raw}, true, nil
	case "customer":
		customer, raw, err := s.Client.GetCustomer(ctx, object.ObjectID)
		if err != nil {
			return domain.ObjectSnapshot{}, false, nil
		}
		return domain.ObjectSnapshot{ObjectType: "customer", ObjectID: customer.ID, Livemode: livemode, Payload: raw}, true, nil
	default:
		return domain.ObjectSnapshot{}, false, nil
	}
}

func explicitVATFromTaxCase(taxCase domain.TaxCase, bt domain.BalanceTransaction) *int64 {
	if !taxCase.StripeTaxAmountKnown || taxCase.StripeTaxAmountMinor == nil {
		return nil
	}
	total := *taxCase.StripeTaxAmountMinor
	if taxCase.SourceCurrency != nil && strings.ToUpper(*taxCase.SourceCurrency) != "SEK" {
		total = amountToSEKOre(total, bt)
	}
	return &total
}
