package notify

import "strings"

func buildBody(notification Notification) string {
	var b strings.Builder
	b.WriteString(notification.Subject)
	b.WriteString("\n\n")
	if notification.PostingDate != nil {
		b.WriteString("Posting date: ")
		b.WriteString(notification.PostingDate.Format("2006-01-02"))
		b.WriteString("\n")
	}
	b.WriteString("Company ID: ")
	b.WriteString(notification.CompanyID)
	b.WriteString("\n")
	b.WriteString("Severity: ")
	b.WriteString(string(notification.Severity))
	b.WriteString("\n")
	b.WriteString("Category: ")
	b.WriteString(string(notification.Category))
	b.WriteString("\n\nSummary\n")
	for _, line := range notification.SummaryLines {
		b.WriteString("- ")
		b.WriteString(line)
		b.WriteString("\n")
	}
	if len(notification.DetailLines) > 0 {
		b.WriteString("\nDetails\n")
		for _, line := range notification.DetailLines {
			b.WriteString("- ")
			b.WriteString(line)
			b.WriteString("\n")
		}
	}
	if len(notification.ActionLines) > 0 {
		b.WriteString("\nHow to handle\n")
		for _, line := range notification.ActionLines {
			b.WriteString("- ")
			b.WriteString(line)
			b.WriteString("\n")
		}
	}
	return b.String()
}
