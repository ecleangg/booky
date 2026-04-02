package stripe

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

var (
	ErrWebhookSignatureInvalid  = errors.New("invalid stripe webhook signature")
	ErrWebhookSignatureMismatch = errors.New("stripe webhook signature mismatch")
	ErrWebhookSignatureExpired  = errors.New("stripe webhook signature expired")
	ErrWebhookSignatureInFuture = errors.New("stripe webhook signature timestamp is too far in the future")
	ErrInvalidEvent             = errors.New("invalid stripe event payload")
)

func (c *Client) VerifyWebhook(payload []byte, signatureHeader string) error {
	return c.VerifyWebhookWithSecret(payload, signatureHeader, c.webhookSecret)
}

func (c *Client) VerifyWebhookWithSecret(payload []byte, signatureHeader, secret string) error {
	if secret == "" {
		return fmt.Errorf("stripe webhook secret is not configured")
	}
	timestamp, signatures, err := parseStripeSignature(signatureHeader)
	if err != nil {
		return err
	}
	signedAt := time.Unix(timestamp, 0)
	if age := time.Since(signedAt); age > 5*time.Minute {
		return ErrWebhookSignatureExpired
	} else if age < -5*time.Minute {
		return ErrWebhookSignatureInFuture
	}
	signedPayload := fmt.Sprintf("%d.%s", timestamp, payload)
	h := hmac.New(sha256.New, []byte(secret))
	_, _ = h.Write([]byte(signedPayload))
	expected := hex.EncodeToString(h.Sum(nil))
	for _, sig := range signatures {
		if subtle.ConstantTimeCompare([]byte(expected), []byte(sig)) == 1 {
			return nil
		}
	}
	return ErrWebhookSignatureMismatch
}

func (c *Client) ParseEvent(payload []byte) (Event, error) {
	var evt Event
	if err := json.Unmarshal(payload, &evt); err != nil {
		return Event{}, fmt.Errorf("decode stripe event: %w", err)
	}
	if evt.ID == "" || evt.Type == "" {
		return Event{}, fmt.Errorf("%w: missing id or type", ErrInvalidEvent)
	}
	return evt, nil
}

func (c *Client) GetCharge(ctx context.Context, id string) (Charge, json.RawMessage, error) {
	var charge Charge
	raw, err := c.getJSON(ctx, "/v1/charges/"+id, &charge)
	if err != nil {
		return Charge{}, nil, err
	}
	return charge, raw, nil
}

func (c *Client) GetPaymentIntent(ctx context.Context, id string) (PaymentIntent, json.RawMessage, error) {
	var paymentIntent PaymentIntent
	raw, err := c.getJSON(ctx, "/v1/payment_intents/"+id, &paymentIntent)
	if err != nil {
		return PaymentIntent{}, nil, err
	}
	return paymentIntent, raw, nil
}

func (c *Client) GetCheckoutSession(ctx context.Context, id string) (CheckoutSession, json.RawMessage, error) {
	var session CheckoutSession
	raw, err := c.getJSON(ctx, "/v1/checkout/sessions/"+id, &session)
	if err != nil {
		return CheckoutSession{}, nil, err
	}
	enriched, err := c.enrichCheckoutSession(ctx, session)
	if err != nil {
		return CheckoutSession{}, nil, err
	}
	enrichedRaw, err := json.Marshal(enriched)
	if err != nil {
		return CheckoutSession{}, nil, fmt.Errorf("encode enriched checkout session: %w", err)
	}
	_ = raw
	return enriched, enrichedRaw, nil
}

func (c *Client) ListCheckoutSessionsByPaymentIntent(ctx context.Context, paymentIntentID string) ([]CheckoutSession, []json.RawMessage, error) {
	params := url.Values{}
	params.Add("limit", "100")
	params.Add("payment_intent", paymentIntentID)
	return c.listCheckoutSessions(ctx, params)
}

func (c *Client) ListCheckoutSessionsBySubscription(ctx context.Context, subscriptionID string) ([]CheckoutSession, []json.RawMessage, error) {
	params := url.Values{}
	params.Add("limit", "100")
	params.Add("subscription", subscriptionID)
	return c.listCheckoutSessions(ctx, params)
}

func (c *Client) GetInvoice(ctx context.Context, id string) (Invoice, json.RawMessage, error) {
	var invoice Invoice
	params := url.Values{}
	params.Add("expand[]", "lines.data.taxes")
	params.Add("expand[]", "lines.data.price.product")
	params.Add("expand[]", "lines.data.pricing.price_details.product")
	raw, err := c.getJSON(ctx, "/v1/invoices/"+id+"?"+params.Encode(), &invoice)
	if err != nil {
		return Invoice{}, nil, err
	}
	return invoice, raw, nil
}

func (c *Client) GetRefund(ctx context.Context, id string) (Refund, json.RawMessage, error) {
	var refund Refund
	raw, err := c.getJSON(ctx, "/v1/refunds/"+id, &refund)
	if err != nil {
		return Refund{}, nil, err
	}
	return refund, raw, nil
}

func (c *Client) GetCustomer(ctx context.Context, id string) (Customer, json.RawMessage, error) {
	var customer Customer
	raw, err := c.getJSON(ctx, "/v1/customers/"+id, &customer)
	if err != nil {
		return Customer{}, nil, err
	}
	return customer, raw, nil
}

func (c *Client) ListCustomerTaxIDs(ctx context.Context, customerID string) ([]TaxID, []json.RawMessage, error) {
	var list taxIDList
	if _, err := c.getJSON(ctx, "/v1/customers/"+customerID+"/tax_ids?limit=100", &list); err != nil {
		return nil, nil, err
	}

	taxIDs := make([]TaxID, 0, len(list.Data))
	raws := make([]json.RawMessage, 0, len(list.Data))
	for _, raw := range list.Data {
		var taxID TaxID
		if err := json.Unmarshal(raw, &taxID); err != nil {
			return nil, nil, fmt.Errorf("decode customer tax id: %w", err)
		}
		taxIDs = append(taxIDs, taxID)
		raws = append(raws, raw)
	}
	return taxIDs, raws, nil
}

func (c *Client) GetPayout(ctx context.Context, id string) (Payout, json.RawMessage, error) {
	var payout Payout
	raw, err := c.getJSON(ctx, "/v1/payouts/"+id, &payout)
	if err != nil {
		return Payout{}, nil, err
	}
	return payout, raw, nil
}

func (c *Client) GetBalanceTransaction(ctx context.Context, id string) (BalanceTransactionAPI, json.RawMessage, error) {
	var bt BalanceTransactionAPI
	raw, err := c.getJSON(ctx, "/v1/balance_transactions/"+id, &bt)
	if err != nil {
		return BalanceTransactionAPI{}, nil, err
	}
	return bt, raw, nil
}

func (c *Client) getJSON(ctx context.Context, path string, out any) (json.RawMessage, error) {
	base, err := url.Parse(c.baseURL)
	if err != nil {
		return nil, fmt.Errorf("parse stripe base url: %w", err)
	}
	ref, err := url.Parse(path)
	if err != nil {
		return nil, fmt.Errorf("parse stripe path: %w", err)
	}
	endpoint := base.ResolveReference(ref)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("create stripe request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	if c.accountID != "" {
		req.Header.Set("Stripe-Account", c.accountID)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call stripe api: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read stripe response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("stripe api %s returned %d: %s", endpoint.Path, resp.StatusCode, bytes.TrimSpace(body))
	}
	if err := json.Unmarshal(body, out); err != nil {
		return nil, fmt.Errorf("decode stripe response: %w", err)
	}
	return body, nil
}

func (c *Client) listCheckoutSessions(ctx context.Context, params url.Values) ([]CheckoutSession, []json.RawMessage, error) {
	var list checkoutSessionList
	raw, err := c.getJSON(ctx, "/v1/checkout/sessions?"+params.Encode(), &list)
	if err != nil {
		return nil, nil, err
	}
	_ = raw
	sessions := make([]CheckoutSession, 0, len(list.Data))
	raws := make([]json.RawMessage, 0, len(list.Data))
	for _, item := range list.Data {
		var session CheckoutSession
		if err := json.Unmarshal(item, &session); err != nil {
			return nil, nil, fmt.Errorf("decode checkout session: %w", err)
		}
		enriched, err := c.enrichCheckoutSession(ctx, session)
		if err != nil {
			return nil, nil, err
		}
		enrichedRaw, err := json.Marshal(enriched)
		if err != nil {
			return nil, nil, fmt.Errorf("encode enriched checkout session: %w", err)
		}
		sessions = append(sessions, enriched)
		raws = append(raws, enrichedRaw)
	}
	return sessions, raws, nil
}

func (c *Client) enrichCheckoutSession(ctx context.Context, session CheckoutSession) (CheckoutSession, error) {
	var lineItems checkoutLineItemList
	params := url.Values{}
	params.Add("limit", "100")
	params.Add("expand[]", "data.price.product")
	if _, err := c.getJSON(ctx, "/v1/checkout/sessions/"+session.ID+"/line_items?"+params.Encode(), &lineItems); err != nil {
		return CheckoutSession{}, err
	}
	session.LineItems = lineItems
	return session, nil
}

func parseStripeSignature(header string) (int64, []string, error) {
	parts := strings.Split(header, ",")
	var timestamp int64
	var signatures []string
	for _, part := range parts {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) != 2 {
			continue
		}
		switch kv[0] {
		case "t":
			value, err := strconv.ParseInt(kv[1], 10, 64)
			if err != nil {
				return 0, nil, fmt.Errorf("invalid stripe signature timestamp: %w", err)
			}
			timestamp = value
		case "v1":
			signatures = append(signatures, kv[1])
		}
	}
	if timestamp == 0 || len(signatures) == 0 {
		return 0, nil, ErrWebhookSignatureInvalid
	}
	return timestamp, signatures, nil
}
