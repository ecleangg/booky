package tax

import (
	"bytes"
	"encoding/json"
)

type address struct {
	Country string `json:"country"`
}

type shipping struct {
	Address address `json:"address"`
}

type taxIDVerification struct {
	Status string `json:"status"`
}

type taxID struct {
	ID           string             `json:"id"`
	Country      string             `json:"country"`
	Type         string             `json:"type"`
	Value        string             `json:"value"`
	Verification *taxIDVerification `json:"verification"`
}

type customerTaxID struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

type customer struct {
	ID        string            `json:"id"`
	Address   *address          `json:"address"`
	Shipping  *shipping         `json:"shipping"`
	TaxExempt string            `json:"tax_exempt"`
	Metadata  map[string]string `json:"metadata"`
}

type customerDetails struct {
	Address address `json:"address"`
	TaxIDs  []taxID `json:"tax_ids"`
}

type productDetails struct {
	ID        string            `json:"id"`
	TaxCode   string            `json:"tax_code"`
	Type      string            `json:"type"`
	Shippable *bool             `json:"shippable"`
	Metadata  map[string]string `json:"metadata"`
}

func (p *productDetails) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, []byte("null")) {
		*p = productDetails{}
		return nil
	}
	var id string
	if err := json.Unmarshal(data, &id); err == nil {
		p.ID = id
		return nil
	}
	type alias productDetails
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*p = productDetails(decoded)
	return nil
}

type priceDetails struct {
	ID       string            `json:"id"`
	Type     string            `json:"type"`
	Product  productDetails    `json:"product"`
	Metadata map[string]string `json:"metadata"`
}

type invoiceAutomaticTax struct {
	Enabled bool   `json:"enabled"`
	Status  string `json:"status"`
}

type invoiceLineTax struct {
	Amount           int64  `json:"amount"`
	TaxabilityReason string `json:"taxability_reason"`
}

type invoiceLine struct {
	Taxes    []invoiceLineTax  `json:"taxes"`
	Metadata map[string]string `json:"metadata"`
	Price    *priceDetails     `json:"price"`
	Pricing  *struct {
		PriceDetails *struct {
			Product productDetails `json:"product"`
		} `json:"price_details"`
	} `json:"pricing"`
}

type invoice struct {
	ID                string              `json:"id"`
	Customer          string              `json:"customer"`
	Currency          string              `json:"currency"`
	Subtotal          int64               `json:"subtotal"`
	Total             int64               `json:"total"`
	CustomerTaxExempt string              `json:"customer_tax_exempt"`
	CustomerAddress   *address            `json:"customer_address"`
	CustomerShipping  *shipping           `json:"customer_shipping"`
	CustomerTaxIDs    []customerTaxID     `json:"customer_tax_ids"`
	AutomaticTax      invoiceAutomaticTax `json:"automatic_tax"`
	Lines             struct {
		Data []invoiceLine `json:"data"`
	} `json:"lines"`
	InvoicePDF string            `json:"invoice_pdf"`
	Status     string            `json:"status"`
	Metadata   map[string]string `json:"metadata"`
}

type checkoutTotalDetails struct {
	AmountTax int64 `json:"amount_tax"`
}

type checkoutSession struct {
	ID              string               `json:"id"`
	Customer        string               `json:"customer"`
	Currency        string               `json:"currency"`
	AmountTotal     int64                `json:"amount_total"`
	CustomerDetails *customerDetails     `json:"customer_details"`
	ShippingDetails *shipping            `json:"shipping_details"`
	AutomaticTax    invoiceAutomaticTax  `json:"automatic_tax"`
	TotalDetails    checkoutTotalDetails `json:"total_details"`
	Invoice         string               `json:"invoice"`
	PaymentIntent   string               `json:"payment_intent"`
	PaymentLink     string               `json:"payment_link"`
	Subscription    string               `json:"subscription"`
	LineItems       struct {
		Data []struct {
			Description string         `json:"description"`
			Price       *priceDetails  `json:"price"`
			Quantity    int64          `json:"quantity"`
			Metadata    map[string]any `json:"metadata"`
		} `json:"data"`
	} `json:"line_items"`
	Metadata map[string]string `json:"metadata"`
}

type stripeRef struct {
	ID string `json:"id"`
}

type paymentIntent struct {
	ID           string            `json:"id"`
	Currency     string            `json:"currency"`
	Amount       int64             `json:"amount"`
	Customer     string            `json:"customer"`
	Metadata     map[string]string `json:"metadata"`
	LatestCharge stripeRef         `json:"latest_charge"`
}

type paymentMethodDetails struct {
	Card *struct {
		Country string `json:"country"`
	} `json:"card"`
}

type charge struct {
	ID                string           `json:"id"`
	Amount            int64            `json:"amount"`
	Currency          string           `json:"currency"`
	Customer          string           `json:"customer"`
	Invoice           string           `json:"invoice"`
	PaymentIntent     string           `json:"payment_intent"`
	CustomerTaxExempt string           `json:"customer_tax_exempt"`
	CustomerDetails   *customerDetails `json:"customer_details"`
	BillingDetails    struct {
		Address address `json:"address"`
	} `json:"billing_details"`
	PaymentMethodDetails *paymentMethodDetails `json:"payment_method_details"`
	Metadata             map[string]string     `json:"metadata"`
	Refunds              struct {
		Data []refund `json:"data"`
	} `json:"refunds"`
}

type refund struct {
	ID       string            `json:"id"`
	Charge   string            `json:"charge"`
	Amount   int64             `json:"amount"`
	Currency string            `json:"currency"`
	Metadata map[string]string `json:"metadata"`
}
