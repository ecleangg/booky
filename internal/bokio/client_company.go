package bokio

import (
	"context"
	"fmt"
	"net/http"
)

func (c *Client) GetCompanyInformation(ctx context.Context) (CompanyInformation, error) {
	var response struct {
		CompanyInformation CompanyInformation `json:"companyInformation"`
	}
	if err := c.doJSON(ctx, http.MethodGet, fmt.Sprintf("/companies/%s/company-information", c.companyID), nil, &response); err != nil {
		return CompanyInformation{}, err
	}
	return response.CompanyInformation, nil
}

func (c *Client) GetChartOfAccounts(ctx context.Context) ([]ChartAccount, error) {
	var response []ChartAccount
	if err := c.doJSON(ctx, http.MethodGet, fmt.Sprintf("/companies/%s/chart-of-accounts", c.companyID), nil, &response); err != nil {
		return nil, err
	}
	return response, nil
}
