package config

import "gopkg.in/yaml.v3"

type FilingsConfig struct {
	Enabled         bool                         `yaml:"enabled" json:"enabled"`
	LeadTimeDays    int                          `yaml:"lead_time_days" json:"lead_time_days"`
	SendTimeLocal   string                       `yaml:"send_time_local" json:"send_time_local"`
	EmailTo         []string                     `yaml:"email_to" json:"email_to"`
	OSSUnion        OSSUnionFilingsConfig        `yaml:"oss_union" json:"oss_union"`
	PeriodicSummary PeriodicSummaryFilingsConfig `yaml:"periodic_summary" json:"periodic_summary"`

	leadTimeDaysSet bool
}

type OSSUnionFilingsConfig struct {
	Enabled          bool   `yaml:"enabled" json:"enabled"`
	IdentifierNumber string `yaml:"identifier_number" json:"identifier_number"`
	OriginCountry    string `yaml:"origin_country" json:"origin_country"`
	ZeroSalesPolicy  string `yaml:"zero_sales_policy" json:"zero_sales_policy"`
}

type PeriodicSummaryFilingsConfig struct {
	Enabled            bool   `yaml:"enabled" json:"enabled"`
	Cadence            string `yaml:"cadence" json:"cadence"`
	ReportingVATNumber string `yaml:"reporting_vat_number" json:"reporting_vat_number"`
	ResponsibleName    string `yaml:"responsible_name" json:"responsible_name"`
	ResponsiblePhone   string `yaml:"responsible_phone" json:"responsible_phone"`
	ResponsibleEmail   string `yaml:"responsible_email" json:"responsible_email"`
}

func (c *FilingsConfig) UnmarshalYAML(value *yaml.Node) error {
	type rawFilingsConfig FilingsConfig

	var raw rawFilingsConfig
	if err := value.Decode(&raw); err != nil {
		return err
	}

	*c = FilingsConfig(raw)
	for i := 0; i+1 < len(value.Content); i += 2 {
		if value.Content[i].Value == "lead_time_days" {
			c.leadTimeDaysSet = true
			break
		}
	}
	return nil
}
