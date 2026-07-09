package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

var (
	ErrDocuSealNotConfigured = errors.New("docuseal is not configured")
	ErrDocuSealNotFound      = errors.New("docuseal record not found")
)

type DocuSealClient struct {
	baseURL    string
	publicURL  string
	apiKey     string
	httpClient *http.Client
}

type DocuSealSubmissionRequest struct {
	TemplateID string
	Submitters []DocuSealSubmitterRequest
}

type DocuSealSubmitterRequest struct {
	Role       string
	Name       string
	Email      string
	ExternalID string
	Values     map[string]string
	Metadata   map[string]string
}

type DocuSealSubmissionResult struct {
	SubmissionID string
	Submitters   []DocuSealSubmitterResult
}

type DocuSealSubmitterResult struct {
	Role        string
	SubmitterID string
	SigningURL  string
}

func (r DocuSealSubmissionResult) SubmitterForRole(role string) (DocuSealSubmitterResult, bool) {
	role = strings.ToLower(strings.TrimSpace(role))
	for _, submitter := range r.Submitters {
		if strings.ToLower(strings.TrimSpace(submitter.Role)) == role {
			return submitter, true
		}
	}
	return DocuSealSubmitterResult{}, false
}

type DocuSealTemplate struct {
	ID         string
	Name       string
	Slug       string
	Archived   bool
	Submitters []string
}

type DocuSealSubmissionStatus struct {
	ID     string
	Status string
}

type DocuSealDocument struct {
	Name string
	URL  string
}

func NewDocuSealClient(cfg Config) *DocuSealClient {
	return &DocuSealClient{
		baseURL:   cfg.DocuSealURL,
		publicURL: cfg.DocuSealPublicURL,
		apiKey:    cfg.DocuSealAPIKey,
		httpClient: &http.Client{
			Timeout: 20 * time.Second,
		},
	}
}

func (c *DocuSealClient) ListTemplates(ctx context.Context) ([]DocuSealTemplate, error) {
	if c.baseURL == "" || c.apiKey == "" {
		return nil, ErrDocuSealNotConfigured
	}

	var templates []DocuSealTemplate
	var after string

	for {
		url := c.baseURL + "/api/templates?limit=100"
		if after != "" {
			url += "&after=" + after
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("X-Auth-Token", c.apiKey)

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, err
		}

		respBody, readErr := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
		resp.Body.Close()
		if readErr != nil {
			return nil, readErr
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, fmt.Errorf("docuseal list templates failed: status %d: %s", resp.StatusCode, string(respBody))
		}

		var page struct {
			Data []struct {
				ID         any     `json:"id"`
				Name       string  `json:"name"`
				Slug       string  `json:"slug"`
				ArchivedAt *string `json:"archived_at"`
				Submitters []struct {
					Name string `json:"name"`
				} `json:"submitters"`
			} `json:"data"`
			Pagination struct {
				Next any `json:"next"`
			} `json:"pagination"`
		}

		if err := json.Unmarshal(respBody, &page); err != nil {
			return nil, err
		}

		for _, item := range page.Data {
			template := DocuSealTemplate{
				ID:       stringID(item.ID),
				Name:     item.Name,
				Slug:     item.Slug,
				Archived: item.ArchivedAt != nil,
			}
			for _, submitter := range item.Submitters {
				if strings.TrimSpace(submitter.Name) != "" {
					template.Submitters = append(template.Submitters, strings.TrimSpace(submitter.Name))
				}
			}
			templates = append(templates, template)
		}

		after = stringID(page.Pagination.Next)
		if after == "" {
			break
		}
	}

	return templates, nil
}

func (c *DocuSealClient) Check(ctx context.Context) error {
	_, err := c.ListTemplates(ctx)
	return err
}

func (c *DocuSealClient) CreateSubmission(ctx context.Context, input DocuSealSubmissionRequest) (DocuSealSubmissionResult, error) {
	if c.baseURL == "" || c.apiKey == "" {
		return DocuSealSubmissionResult{}, ErrDocuSealNotConfigured
	}

	submitters := make([]map[string]any, 0, len(input.Submitters))
	for _, submitter := range input.Submitters {
		submitters = append(submitters, map[string]any{
			"name":        submitter.Name,
			"email":       submitter.Email,
			"role":        submitter.Role,
			"external_id": submitter.ExternalID,
			"values":      submitter.Values,
			"metadata":    submitter.Metadata,
		})
	}

	payload := map[string]any{
		"template_id": input.TemplateID,
		"send_email":  false,
		"send_sms":    false,
		"submitters":  submitters,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return DocuSealSubmissionResult{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/submissions/init", bytes.NewReader(body))
	if err != nil {
		return DocuSealSubmissionResult{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Auth-Token", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return DocuSealSubmissionResult{}, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return DocuSealSubmissionResult{}, err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return DocuSealSubmissionResult{}, fmt.Errorf("docuseal create submission failed: status %d: %s", resp.StatusCode, string(respBody))
	}

	var created struct {
		ID         any `json:"id"`
		Submitters []struct {
			ID           any    `json:"id"`
			SubmissionID any    `json:"submission_id"`
			EmbedSrc     string `json:"embed_src"`
			Role         string `json:"role"`
		} `json:"submitters"`
	}

	if err := json.Unmarshal(respBody, &created); err != nil {
		return DocuSealSubmissionResult{}, err
	}

	result := DocuSealSubmissionResult{
		SubmissionID: stringID(created.ID),
	}

	for index, submitter := range created.Submitters {
		if result.SubmissionID == "" {
			result.SubmissionID = stringID(submitter.SubmissionID)
		}

		role := strings.TrimSpace(submitter.Role)
		if role == "" && index < len(input.Submitters) {
			role = input.Submitters[index].Role
		}

		result.Submitters = append(result.Submitters, DocuSealSubmitterResult{
			Role:        role,
			SubmitterID: stringID(submitter.ID),
			SigningURL:  c.publicSigningURL(submitter.EmbedSrc),
		})
	}

	if result.SubmissionID == "" || len(result.Submitters) == 0 {
		return DocuSealSubmissionResult{}, fmt.Errorf("docuseal create submission response missing required fields")
	}
	for _, submitter := range result.Submitters {
		if submitter.SubmitterID == "" || submitter.SigningURL == "" {
			return DocuSealSubmissionResult{}, fmt.Errorf("docuseal create submission response missing required fields")
		}
	}

	return result, nil
}

func (c *DocuSealClient) UpdateSubmitter(ctx context.Context, submitterID string, input DocuSealSubmitterRequest) (DocuSealSubmitterResult, error) {
	if c.baseURL == "" || c.apiKey == "" {
		return DocuSealSubmitterResult{}, ErrDocuSealNotConfigured
	}

	payload := map[string]any{
		"name":        input.Name,
		"email":       input.Email,
		"external_id": input.ExternalID,
		"metadata":    input.Metadata,
		"send_email":  false,
		"send_sms":    false,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return DocuSealSubmitterResult{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, c.baseURL+"/api/submitters/"+url.PathEscape(submitterID), bytes.NewReader(body))
	if err != nil {
		return DocuSealSubmitterResult{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Auth-Token", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return DocuSealSubmitterResult{}, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return DocuSealSubmitterResult{}, err
	}

	if resp.StatusCode == http.StatusNotFound {
		return DocuSealSubmitterResult{}, ErrDocuSealNotFound
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return DocuSealSubmitterResult{}, fmt.Errorf("docuseal update submitter failed: status %d: %s", resp.StatusCode, string(respBody))
	}

	var updated struct {
		ID       any    `json:"id"`
		EmbedSrc string `json:"embed_src"`
		Role     string `json:"role"`
	}
	if err := json.Unmarshal(respBody, &updated); err != nil {
		return DocuSealSubmitterResult{}, err
	}

	result := DocuSealSubmitterResult{
		Role:        strings.TrimSpace(updated.Role),
		SubmitterID: stringID(updated.ID),
		SigningURL:  c.publicSigningURL(updated.EmbedSrc),
	}
	if result.SubmitterID == "" || result.SigningURL == "" {
		return DocuSealSubmitterResult{}, fmt.Errorf("docuseal update submitter response missing required fields")
	}

	return result, nil
}

func (c *DocuSealClient) GetSubmissionStatus(ctx context.Context, submissionID string) (DocuSealSubmissionStatus, error) {
	if c.baseURL == "" || c.apiKey == "" {
		return DocuSealSubmissionStatus{}, ErrDocuSealNotConfigured
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/submissions/"+submissionID, nil)
	if err != nil {
		return DocuSealSubmissionStatus{}, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Auth-Token", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return DocuSealSubmissionStatus{}, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return DocuSealSubmissionStatus{}, err
	}

	if resp.StatusCode == http.StatusNotFound {
		return DocuSealSubmissionStatus{}, ErrDocuSealNotFound
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return DocuSealSubmissionStatus{}, fmt.Errorf("docuseal get submission failed: status %d: %s", resp.StatusCode, string(respBody))
	}

	var submission struct {
		ID         any     `json:"id"`
		Status     string  `json:"status"`
		ArchivedAt *string `json:"archived_at"`
	}
	if err := json.Unmarshal(respBody, &submission); err != nil {
		return DocuSealSubmissionStatus{}, err
	}

	return DocuSealSubmissionStatus{
		ID:     stringID(submission.ID),
		Status: normalizeDocuSealStatus(submission.Status, submission.ArchivedAt != nil),
	}, nil
}

func (c *DocuSealClient) ArchiveSubmission(ctx context.Context, submissionID string) error {
	if c.baseURL == "" || c.apiKey == "" {
		return ErrDocuSealNotConfigured
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.baseURL+"/api/submissions/"+url.PathEscape(submissionID), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Auth-Token", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return err
	}

	if resp.StatusCode == http.StatusNotFound {
		return ErrDocuSealNotFound
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("docuseal archive submission failed: status %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

func (c *DocuSealClient) GetSubmissionDocuments(ctx context.Context, submissionID string, merge bool) ([]DocuSealDocument, error) {
	if c.baseURL == "" || c.apiKey == "" {
		return nil, ErrDocuSealNotConfigured
	}

	endpoint := c.baseURL + "/api/submissions/" + url.PathEscape(submissionID) + "/documents"
	if merge {
		endpoint += "?merge=true"
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Auth-Token", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrDocuSealNotFound
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("docuseal get submission documents failed: status %d: %s", resp.StatusCode, string(respBody))
	}

	var output struct {
		Documents []struct {
			Name string `json:"name"`
			URL  string `json:"url"`
		} `json:"documents"`
	}
	if err := json.Unmarshal(respBody, &output); err != nil {
		return nil, err
	}

	documents := make([]DocuSealDocument, 0, len(output.Documents))
	for _, document := range output.Documents {
		document.URL = c.publicSigningURL(document.URL)
		if strings.TrimSpace(document.URL) == "" {
			continue
		}
		documents = append(documents, DocuSealDocument{
			Name: strings.TrimSpace(document.Name),
			URL:  strings.TrimSpace(document.URL),
		})
	}
	return documents, nil
}

func (c *DocuSealClient) DownloadDocument(ctx context.Context, documentURL string) ([]byte, string, error) {
	if strings.TrimSpace(documentURL) == "" {
		return nil, "", ErrDocuSealNotFound
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, documentURL, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("Accept", "application/pdf")
	if c.apiKey != "" {
		req.Header.Set("X-Auth-Token", c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, "", ErrDocuSealNotFound
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, "", fmt.Errorf("docuseal document download failed: status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return nil, "", err
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/pdf"
	}
	return body, contentType, nil
}

func (c *DocuSealClient) publicSigningURL(signingURL string) string {
	if c.publicURL == "" || c.publicURL == c.baseURL {
		return signingURL
	}

	return strings.Replace(signingURL, c.baseURL, c.publicURL, 1)
}

func stringID(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case float64:
		return strconv.FormatInt(int64(v), 10)
	case nil:
		return ""
	default:
		return fmt.Sprintf("%v", v)
	}
}

func normalizeDocuSealStatus(status string, archived bool) string {
	if archived {
		return "canceled"
	}

	switch strings.ToLower(strings.TrimSpace(status)) {
	case "completed", "complete":
		return "complete"
	case "declined", "canceled", "cancelled":
		return "canceled"
	case "expired", "failed":
		return "failed"
	default:
		return "pending"
	}
}
