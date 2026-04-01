package bokio

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"strings"

	"github.com/google/uuid"
)

func (c *Client) FindUploadByJournalEntryAndDescription(ctx context.Context, journalEntryID uuid.UUID, description string) (*uuid.UUID, error) {
	uploads, err := c.listUploads(ctx, fmt.Sprintf("journalEntryId==%s", journalEntryID), 100)
	if err != nil {
		return nil, err
	}
	for _, upload := range uploads {
		if upload.JournalEntryID == journalEntryID && upload.Description == description {
			id := upload.ID
			return &id, nil
		}
	}
	return nil, nil
}

func (c *Client) UploadJournalAttachment(ctx context.Context, journalEntryID uuid.UUID, description, filename string, data []byte) (uuid.UUID, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	partHeader := textproto.MIMEHeader{}
	partHeader.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename="%s"`, filename))
	partHeader.Set("Content-Type", "application/pdf")

	fileWriter, err := writer.CreatePart(partHeader)
	if err != nil {
		return uuid.Nil, fmt.Errorf("create multipart file field: %w", err)
	}
	if _, err := fileWriter.Write(data); err != nil {
		return uuid.Nil, fmt.Errorf("write multipart file data: %w", err)
	}
	if err := writer.WriteField("journalEntryId", journalEntryID.String()); err != nil {
		return uuid.Nil, fmt.Errorf("write journalEntryId field: %w", err)
	}
	if err := writer.WriteField("description", description); err != nil {
		return uuid.Nil, fmt.Errorf("write description field: %w", err)
	}
	if err := writer.Close(); err != nil {
		return uuid.Nil, fmt.Errorf("close multipart writer: %w", err)
	}

	endpoint := fmt.Sprintf("/companies/%s/uploads", c.companyID)
	fullURL, err := c.resolveURL(endpoint)
	if err != nil {
		return uuid.Nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fullURL, &body)
	if err != nil {
		return uuid.Nil, fmt.Errorf("create upload request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return uuid.Nil, fmt.Errorf("call bokio upload api: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return uuid.Nil, fmt.Errorf("read bokio upload response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return uuid.Nil, fmt.Errorf("bokio uploads returned %d: %s", resp.StatusCode, bytes.TrimSpace(bodyBytes))
	}
	var upload struct {
		ID uuid.UUID `json:"id"`
	}
	if err := json.Unmarshal(bodyBytes, &upload); err != nil {
		return uuid.Nil, fmt.Errorf("decode bokio upload response: %w", err)
	}
	if upload.ID == uuid.Nil {
		return uuid.Nil, fmt.Errorf("bokio upload response missing id")
	}
	return upload.ID, nil
}

func (c *Client) listUploads(ctx context.Context, query string, pageSize int) ([]Upload, error) {
	endpoint := fmt.Sprintf("/companies/%s/uploads?page=1&pageSize=%d", c.companyID, pageSize)
	if strings.TrimSpace(query) != "" {
		endpoint += "&query=" + url.QueryEscape(query)
	}
	var response struct {
		Items []Upload `json:"items"`
	}
	if err := c.doJSON(ctx, http.MethodGet, endpoint, nil, &response); err != nil {
		return nil, err
	}
	return response.Items, nil
}
