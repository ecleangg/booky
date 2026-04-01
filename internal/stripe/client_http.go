package stripe

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

func (c *Client) VerifyWebhook(payload []byte, signatureHeader string) error {
	if c.webhookSecret == "" {
		return fmt.Errorf("stripe webhook secret is not configured")
	}
	timestamp, signatures, err := parseStripeSignature(signatureHeader)
	if err != nil {
		return err
	}
	if time.Since(time.Unix(timestamp, 0)) > 5*time.Minute {
		return fmt.Errorf("stripe webhook signature is too old")
	}
	signedPayload := fmt.Sprintf("%d.%s", timestamp, payload)
	h := hmac.New(sha256.New, []byte(c.webhookSecret))
	_, _ = h.Write([]byte(signedPayload))
	expected := hex.EncodeToString(h.Sum(nil))
	for _, sig := range signatures {
		if subtle.ConstantTimeCompare([]byte(expected), []byte(sig)) == 1 {
			return nil
		}
	}
	return fmt.Errorf("stripe webhook signature mismatch")
}

func (c *Client) ParseEvent(payload []byte) (Event, error) {
	var evt Event
	if err := json.Unmarshal(payload, &evt); err != nil {
		return Event{}, fmt.Errorf("decode stripe event: %w", err)
	}
	if evt.ID == "" || evt.Type == "" {
		return Event{}, fmt.Errorf("stripe event missing id or type")
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

func (c *Client) GetInvoice(ctx context.Context, id string) (Invoice, json.RawMessage, error) {
	var invoice Invoice
	params := url.Values{}
	params.Add("expand[]", "lines.data.taxes")
	raw, err := c.getJSON(ctx, "/v1/invoices/"+id+"?"+params.Encode(), &invoice)
	if err != nil {
		return Invoice{}, nil, err
	}
	return invoice, raw, nil
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
		return 0, nil, fmt.Errorf("invalid stripe signature header")
	}
	return timestamp, signatures, nil
}
