package config

import "gopkg.in/yaml.v3"

type FilingsConfig struct {
	Enabled         bool                         `yaml:"enabled"`
	LeadTimeDays    int                          `yaml:"lead_time_days"`
	SendTimeLocal   string                       `yaml:"send_time_local"`
	EmailTo         []string                     `yaml:"email_to"`
	OSSUnion        OSSUnionFilingsConfig        `yaml:"oss_union"`
	PeriodicSummary PeriodicSummaryFilingsConfig `yaml:"periodic_summary"`

	leadTimeDaysSet bool
}

type OSSUnionFilingsConfig struct {
	Enabled          bool   `yaml:"enabled"`
	IdentifierNumber string `yaml:"identifier_number"`
	OriginCountry    string `yaml:"origin_country"`
	ZeroSalesPolicy  string `yaml:"zero_sales_policy"`
}

type PeriodicSummaryFilingsConfig struct {
	Enabled            bool   `yaml:"enabled"`
	Cadence            string `yaml:"cadence"`
	ReportingVATNumber string `yaml:"reporting_vat_number"`
	ResponsibleName    string `yaml:"responsible_name"`
	ResponsiblePhone   string `yaml:"responsible_phone"`
	ResponsibleEmail   string `yaml:"responsible_email"`
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
