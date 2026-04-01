package bokio

import (
	"context"
	"fmt"
	"net/http"
)

func (c *Client) Check(ctx context.Context) (CheckResult, error) {
	result := CheckResult{CompanyID: c.companyID}

	var companyResp struct {
		CompanyInformation struct {
			Name string `json:"name"`
		} `json:"companyInformation"`
	}
	if err := c.doJSON(ctx, http.MethodGet, fmt.Sprintf("/companies/%s/company-information", c.companyID), nil, &companyResp); err != nil {
		return result, err
	}
	result.CompanyInformation = true
	result.CompanyName = companyResp.CompanyInformation.Name

	var chartResp []struct {
		Account int `json:"account"`
	}
	if err := c.doJSON(ctx, http.MethodGet, fmt.Sprintf("/companies/%s/chart-of-accounts", c.companyID), nil, &chartResp); err != nil {
		return result, err
	}
	result.ChartOfAccounts = true

	var fiscalResp struct {
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
	}
	if err := c.doJSON(ctx, http.MethodGet, fmt.Sprintf("/companies/%s/fiscal-years", c.companyID), nil, &fiscalResp); err != nil {
		return result, err
	}
	result.FiscalYears = true

	return result, nil
}
