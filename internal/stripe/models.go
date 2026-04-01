package stripe

import (
	"bytes"
	"encoding/json"
)

type Event struct {
	ID         string `json:"id"`
	Type       string `json:"type"`
	APIVersion string `json:"api_version"`
	Created    int64  `json:"created"`
	Livemode   bool   `json:"livemode"`
	Account    string `json:"account"`
	Data       struct {
		Object json.RawMessage `json:"object"`
	} `json:"data"`
}

type PaymentIntent struct {
	ID           string            `json:"id"`
	Created      int64             `json:"created"`
	Currency     string            `json:"currency"`
	Customer     string            `json:"customer"`
	Metadata     map[string]string `json:"metadata"`
	LatestCharge StripeRef         `json:"latest_charge"`
}

type StripeRef struct {
	ID     string `json:"id"`
	Object string `json:"object"`
}

func (r *StripeRef) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, []byte("null")) {
		return nil
	}
	var id string
	if err := json.Unmarshal(data, &id); err == nil {
		r.ID = id
		return nil
	}
	var obj struct {
		ID     string `json:"id"`
		Object string `json:"object"`
	}
	if err := json.Unmarshal(data, &obj); err != nil {
		return err
	}
	r.ID = obj.ID
	r.Object = obj.Object
	return nil
}

type Charge struct {
	ID                 string            `json:"id"`
	Amount             int64             `json:"amount"`
	Currency           string            `json:"currency"`
	Created            int64             `json:"created"`
	BalanceTransaction any               `json:"balance_transaction"`
	BillingDetails     billingDetails    `json:"billing_details"`
	CustomerDetails    *customerDetails  `json:"customer_details"`
	CustomerTaxExempt  string            `json:"customer_tax_exempt"`
	Customer           string            `json:"customer"`
	Metadata           map[string]string `json:"metadata"`
	Refunds            struct {
		Data []Refund `json:"data"`
	} `json:"refunds"`
	PaymentIntent string `json:"payment_intent"`
	Invoice       string `json:"invoice"`
}

type Invoice struct {
	ID                string                 `json:"id"`
	Customer          string                 `json:"customer"`
	CustomerTaxExempt string                 `json:"customer_tax_exempt"`
	CustomerAddress   *address               `json:"customer_address"`
	CustomerShipping  *shipping              `json:"customer_shipping"`
	CustomerTaxIDs    []InvoiceCustomerTaxID `json:"customer_tax_ids"`
	AutomaticTax      invoiceAutomaticTax    `json:"automatic_tax"`
	Lines             invoiceLineCollection  `json:"lines"`
	Metadata          map[string]string      `json:"metadata"`
}

type InvoiceCustomerTaxID struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

type invoiceAutomaticTax struct {
	Enabled bool   `json:"enabled"`
	Status  string `json:"status"`
}

type invoiceLineCollection struct {
	Data []InvoiceLine `json:"data"`
}

type InvoiceLine struct {
	Taxes []InvoiceLineTax `json:"taxes"`
}

type InvoiceLineTax struct {
	Amount           int64  `json:"amount"`
	TaxabilityReason string `json:"taxability_reason"`
}

type Customer struct {
	ID        string            `json:"id"`
	Address   *address          `json:"address"`
	Shipping  *shipping         `json:"shipping"`
	TaxExempt string            `json:"tax_exempt"`
	Metadata  map[string]string `json:"metadata"`
}

type TaxID struct {
	ID           string             `json:"id"`
	Country      string             `json:"country"`
	Type         string             `json:"type"`
	Value        string             `json:"value"`
	Verification *taxIDVerification `json:"verification"`
}

type taxIDVerification struct {
	Status string `json:"status"`
}

type taxIDList struct {
	Data []json.RawMessage `json:"data"`
}

type Refund struct {
	ID                 string            `json:"id"`
	Amount             int64             `json:"amount"`
	Currency           string            `json:"currency"`
	Created            int64             `json:"created"`
	BalanceTransaction any               `json:"balance_transaction"`
	Metadata           map[string]string `json:"metadata"`
}

type Dispute struct {
	ID                  string            `json:"id"`
	Amount              int64             `json:"amount"`
	Currency            string            `json:"currency"`
	Created             int64             `json:"created"`
	Charge              string            `json:"charge"`
	Metadata            map[string]string `json:"metadata"`
	BalanceTransactions []struct {
		ID string `json:"id"`
	} `json:"balance_transactions"`
}

type Payout struct {
	ID                 string            `json:"id"`
	Amount             int64             `json:"amount"`
	Currency           string            `json:"currency"`
	Created            int64             `json:"created"`
	ArrivalDate        int64             `json:"arrival_date"`
	BalanceTransaction any               `json:"balance_transaction"`
	Metadata           map[string]string `json:"metadata"`
}

type BalanceTransactionAPI struct {
	ID                string    `json:"id"`
	Amount            int64     `json:"amount"`
	Fee               int64     `json:"fee"`
	Net               int64     `json:"net"`
	Currency          string    `json:"currency"`
	Type              string    `json:"type"`
	ReportingCategory string    `json:"reporting_category"`
	Status            string    `json:"status"`
	ExchangeRate      *float64  `json:"exchange_rate"`
	AvailableOn       int64     `json:"available_on"`
	Created           int64     `json:"created"`
	Source            StripeRef `json:"source"`
}

type billingDetails struct {
	Address address `json:"address"`
}

type customerDetails struct {
	Address address `json:"address"`
}

type shipping struct {
	Address address `json:"address"`
}

type address struct {
	Country string `json:"country"`
}

type chargeEvidenceBundle struct {
	Invoice        *Invoice
	Customer       *Customer
	CustomerTaxIDs []TaxID
}
