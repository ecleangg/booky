package config

type FilingsConfig struct {
	Enabled         bool                         `yaml:"enabled"`
	LeadTimeDays    int                          `yaml:"lead_time_days"`
	SendTimeLocal   string                       `yaml:"send_time_local"`
	EmailTo         []string                     `yaml:"email_to"`
	OSSUnion        OSSUnionFilingsConfig        `yaml:"oss_union"`
	PeriodicSummary PeriodicSummaryFilingsConfig `yaml:"periodic_summary"`
	FX              FilingFXConfig               `yaml:"fx"`
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

type FilingFXConfig struct {
	OSSProvider     string `yaml:"oss_provider"`
	PSProvider      string `yaml:"ps_provider"`
	ECBBaseURL      string `yaml:"ecb_base_url"`
	RiksbankBaseURL string `yaml:"riksbank_base_url"`
	RiksbankAPIKey  string `yaml:"riksbank_api_key"`
}
