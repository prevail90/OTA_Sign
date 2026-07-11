package app

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type Server struct {
	cfg           Config
	store         *Store
	docuseal      *DocuSealClient
	notifications *NotificationClient
}

func NewServer(cfg Config) (*Server, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	store, err := NewStore(context.Background(), cfg)
	if err != nil {
		return nil, err
	}

	return &Server{
		cfg:           cfg,
		store:         store,
		docuseal:      NewDocuSealClient(cfg),
		notifications: NewNotificationClient(cfg),
	}, nil
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", s.handleHealth)
	mux.HandleFunc("GET /readyz", s.handleReady)
	mux.HandleFunc("GET /healthz/full", s.handleFullHealth)
	mux.HandleFunc("GET /launch", s.handleLaunch)
	mux.HandleFunc("POST /webhooks/docuseal", s.handleDocuSealWebhook)
	mux.HandleFunc("POST /dev/launch-token", s.handleDevLaunchToken)
	mux.HandleFunc("GET /api/me", s.withAuth(s.handleMe))
	mux.HandleFunc("GET /api/templates", s.withAuth(s.handleTemplates))
	mux.HandleFunc("POST /api/submissions", s.withAuth(s.handleCreateSubmission))
	mux.HandleFunc("POST /api/submissions/{id}/commander-sign", s.withAuth(s.handleCommanderSign))
	mux.HandleFunc("GET /api/submissions/{id}/download", s.withAuth(s.handleDownloadSubmission))
	mux.HandleFunc("DELETE /api/submissions/{id}", s.withAuth(s.handleDeleteSubmission))
	mux.HandleFunc("GET /api/my/submissions", s.withAuth(s.handleMySubmissions))
	mux.HandleFunc("GET /api/unit/submissions", s.withAuth(s.handleUnitSubmissions))

	return s.withHTTPS(s.withCORS(mux))
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	checks := map[string]string{"database": "ok"}
	if err := s.store.Check(ctx); err != nil {
		checks["database"] = "error"
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"status": "error",
			"checks": checks,
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"checks": checks,
	})
}

func (s *Server) handleFullHealth(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()

	statusCode := http.StatusOK
	checks := map[string]string{
		"database": "ok",
		"docuseal": "ok",
	}

	if err := s.store.Check(ctx); err != nil {
		checks["database"] = "error"
		statusCode = http.StatusServiceUnavailable
	}

	if err := s.docuseal.Check(ctx); err != nil {
		if errors.Is(err, ErrDocuSealNotConfigured) {
			checks["docuseal"] = "not_configured"
		} else {
			checks["docuseal"] = "error"
			statusCode = http.StatusServiceUnavailable
		}
	}

	status := "ok"
	if statusCode != http.StatusOK {
		status = "error"
	}

	writeJSON(w, statusCode, map[string]any{
		"status": status,
		"checks": checks,
	})
}

func (s *Server) handleLaunch(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	claims, err := VerifyLaunchToken(token, s.cfg.MoodleLaunchSigningSecret, time.Now().UTC())
	if err != nil {
		log.Printf("moodle launch rejected: %v", err)
		http.Redirect(w, r, s.cfg.MoodleLoginURL, http.StatusFound)
		return
	}

	user := User{
		ID:            "moodle-" + claims.MoodleUserID,
		MoodleUserID:  claims.MoodleUserID,
		FullName:      formattedMilitaryName(claims),
		FirstName:     claims.FirstName,
		LastName:      claims.LastName,
		MiddleInitial: claims.MiddleInitial,
		Email:         claims.Email,
		DoDID:         claims.DoDID,
		Rank:          claims.Rank,
		UIC:           claims.UIC,
		Roles:         claims.Roles,
		Capabilities:  claims.Capabilities,
	}
	s.enrichPayGradeFromRank(r.Context(), &user)

	sessionID := randomID()
	if err := s.store.SaveSession(r.Context(), sessionID, user); err != nil {
		log.Printf("save launch session failed: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not start session"})
		return
	}
	s.setSessionCookie(w, sessionID)

	http.Redirect(w, r, s.cfg.FrontendURL+"/dashboard", http.StatusFound)
}

func (s *Server) handleDevLaunchToken(w http.ResponseWriter, r *http.Request) {
	if strings.ToLower(s.cfg.MoodleLaunchSigningSecret) != "dev-only-change-me" {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "disabled"})
		return
	}

	now := time.Now().UTC()
	claims := LaunchClaims{
		MoodleUserID: "1001",
		FullName:     "Demo Soldier",
		FirstName:    "Demo",
		LastName:     "Soldier",
		Email:        "demo.soldier@example.mil",
		DoDID:        "1111222233",
		Rank:         "SGT",
		UIC:          "WABC12",
		Roles:        []string{"student", "commander"},
		Capabilities: []string{"viewown", "viewunit", "signascommander"},
		IssuedAt:     now,
		ExpiresAt:    now.Add(5 * time.Minute),
	}

	token, err := SignLaunchClaims(claims, s.cfg.MoodleLaunchSigningSecret)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not sign token"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"token":      token,
		"launch_url": "/launch?token=" + token,
	})
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request, user User) {
	writeJSON(w, http.StatusOK, user)
}

func (s *Server) handleTemplates(w http.ResponseWriter, r *http.Request, user User) {
	s.syncDocuSealTemplates(r.Context())

	templates, err := s.store.TemplatesForUser(r.Context(), user)
	if err != nil {
		log.Printf("load templates failed: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not load templates"})
		return
	}

	writeJSON(w, http.StatusOK, templates)
}

func (s *Server) syncDocuSealTemplates(ctx context.Context) {
	docusealTemplates, err := s.docuseal.ListTemplates(ctx)
	if errors.Is(err, ErrDocuSealNotConfigured) {
		return
	}
	if err != nil {
		log.Printf("docuseal template sync failed: %v", err)
		return
	}
	if err := s.store.SyncDocuSealTemplates(ctx, docusealTemplates); err != nil {
		log.Printf("store docuseal template sync failed: %v", err)
	}
}

func (s *Server) handleMySubmissions(w http.ResponseWriter, r *http.Request, user User) {
	s.refreshPendingSubmissions(r.Context(), user)

	submissions, err := s.store.MySubmissions(r.Context(), user)
	if err != nil {
		log.Printf("load my submissions failed: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not load submissions"})
		return
	}

	writeJSON(w, http.StatusOK, submissions)
}

func (s *Server) handleCreateSubmission(w http.ResponseWriter, r *http.Request, user User) {
	var input struct {
		TemplateID string `json:"template_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if input.TemplateID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "template_id is required"})
		return
	}

	template, ok, err := s.store.TemplateByID(r.Context(), input.TemplateID)
	if err != nil {
		log.Printf("load template failed: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not load template"})
		return
	}
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "template not found"})
		return
	}

	prefillUser := user
	s.enrichPayGradeFromRank(r.Context(), &prefillUser)

	docusealSubmitters := []DocuSealSubmitterRequest{
		{
			Role:       template.SoldierRoleName,
			Name:       prefillUser.FullName,
			Email:      user.Email,
			ExternalID: user.ID,
			Values:     docusealPrefillValues(prefillUser),
			Metadata: map[string]string{
				"otasign_user_id":     user.ID,
				"moodle_user_id":      user.MoodleUserID,
				"uic":                 user.UIC,
				"otasign_template_id": template.ID,
				"otasign_signer_role": template.SoldierRoleName,
			},
		},
	}
	if template.RequiresCommanderSignature && template.CommanderRoleName != "" {
		docusealSubmitters = append(docusealSubmitters, DocuSealSubmitterRequest{
			Role:       template.CommanderRoleName,
			Name:       "Pending Commander",
			Email:      "pending-commander+" + randomID() + "@example.invalid",
			ExternalID: "pending-commander:" + user.UIC + ":" + template.ID,
			Metadata: map[string]string{
				"uic":                 user.UIC,
				"otasign_template_id": template.ID,
				"otasign_signer_role": template.CommanderRoleName,
			},
		})
	}

	docusealResult, err := s.docuseal.CreateSubmission(r.Context(), DocuSealSubmissionRequest{
		TemplateID: template.DocuSealTemplateID,
		Submitters: docusealSubmitters,
	})
	if errors.Is(err, ErrDocuSealNotConfigured) {
		writeJSON(w, http.StatusFailedDependency, map[string]string{"error": "docuseal is not configured"})
		return
	}
	if err != nil {
		log.Printf("docuseal create submission failed: %v", err)
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "could not create docuseal submission"})
		return
	}

	submission, err := s.store.CreateSubmission(r.Context(), user, template, docusealResult)
	if err != nil {
		log.Printf("store submission failed: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not store submission"})
		return
	}

	writeJSON(w, http.StatusCreated, submission)
}

func (s *Server) handleDownloadSubmission(w http.ResponseWriter, r *http.Request, user User) {
	submissionID := strings.TrimSpace(r.PathValue("id"))
	if submissionID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "submission id is required"})
		return
	}

	download, ok, err := s.store.SubmissionDownloadForUser(r.Context(), submissionID, user)
	if err != nil {
		log.Printf("load submission download failed: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not load submission"})
		return
	}
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "submission not found"})
		return
	}
	if download.Status != "complete" {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "submission is not complete"})
		return
	}
	if download.DocuSealSubmissionID == "" {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "docuseal submission not found"})
		return
	}

	documents, err := s.docuseal.GetSubmissionDocuments(r.Context(), download.DocuSealSubmissionID, true)
	if errors.Is(err, ErrDocuSealNotConfigured) {
		writeJSON(w, http.StatusFailedDependency, map[string]string{"error": "docuseal is not configured"})
		return
	}
	if errors.Is(err, ErrDocuSealNotFound) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "docuseal documents not found"})
		return
	}
	if err != nil {
		log.Printf("docuseal get documents failed: %v", err)
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "could not load docuseal documents"})
		return
	}
	if len(documents) == 0 {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "docuseal documents not found"})
		return
	}

	body, contentType, err := s.docuseal.DownloadDocument(r.Context(), documents[0].URL)
	if errors.Is(err, ErrDocuSealNotFound) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "docuseal document not found"})
		return
	}
	if err != nil {
		log.Printf("docuseal download document failed: %v", err)
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "could not download docuseal document"})
		return
	}

	filename := safePDFName(download.TemplateName)
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	w.Header().Set("Cache-Control", "private, no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

func (s *Server) handleCommanderSign(w http.ResponseWriter, r *http.Request, user User) {
	if !hasAny(user.Capabilities, "signascommander") {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "not authorized to sign as commander"})
		return
	}

	submissionID := strings.TrimSpace(r.PathValue("id"))
	if submissionID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "submission id is required"})
		return
	}

	submission, docusealSubmitterID, ok, err := s.store.CommanderSignerForSubmission(r.Context(), submissionID, user)
	if err != nil {
		log.Printf("load commander signer failed: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not load commander signer"})
		return
	}
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "commander signature is not available"})
		return
	}

	result, err := s.docuseal.UpdateSubmitter(r.Context(), docusealSubmitterID, DocuSealSubmitterRequest{
		Name:       user.FullName,
		Email:      user.Email,
		ExternalID: user.ID,
		Metadata: map[string]string{
			"otasign_user_id":       user.ID,
			"moodle_user_id":        user.MoodleUserID,
			"uic":                   user.UIC,
			"otasign_submission_id": submission.ID,
			"otasign_template_id":   submission.TemplateID,
			"otasign_signer_role":   "commander",
		},
	})
	if errors.Is(err, ErrDocuSealNotConfigured) {
		writeJSON(w, http.StatusFailedDependency, map[string]string{"error": "docuseal is not configured"})
		return
	}
	if errors.Is(err, ErrDocuSealNotFound) {
		if err := s.store.CancelSubmission(r.Context(), submission.ID); err != nil {
			log.Printf("cancel missing commander submission failed: %v", err)
		}
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "docuseal signer not found"})
		return
	}
	if err != nil {
		log.Printf("docuseal update commander submitter failed: %v", err)
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "could not prepare commander signer"})
		return
	}

	if err := s.store.AssignCommanderSigner(r.Context(), submission.ID, user, docusealSubmitterID, result.SigningURL); err != nil {
		log.Printf("assign commander signer failed: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not assign commander signer"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"signing_url": result.SigningURL})
}

func (s *Server) handleDocuSealWebhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 4<<20))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "could not read webhook body"})
		return
	}

	if err := s.verifyDocuSealWebhookSignature(r.Header.Get("X-Docuseal-Signature"), body); err != nil {
		log.Printf("docuseal webhook rejected: %v", err)
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid webhook signature"})
		return
	}

	var event struct {
		EventID   string          `json:"event_id"`
		EventType string          `json:"event_type"`
		Timestamp json.RawMessage `json:"timestamp"`
		Data      map[string]any  `json:"data"`
	}
	if err := json.Unmarshal(body, &event); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid webhook body"})
		return
	}
	event.EventType = strings.TrimSpace(event.EventType)
	if event.EventType == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "event_type is required"})
		return
	}

	docusealSubmissionID := docusealSubmissionIDFromWebhook(event.EventType, event.Data)
	if err := s.store.RecordDocuSealWebhookEvent(r.Context(), event.EventID, event.EventType, docusealSubmissionID, body); err != nil {
		log.Printf("record docuseal webhook event failed: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not record webhook event"})
		return
	}

	if err := s.applyDocuSealWebhookEvent(r.Context(), event.EventType, event.Data); err != nil {
		log.Printf("process docuseal webhook event failed: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not process webhook event"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "processed"})
}

func (s *Server) verifyDocuSealWebhookSignature(header string, body []byte) error {
	if s.cfg.DocuSealWebhookSecret == "" {
		return errors.New("DOCUSEAL_WEBHOOK_SECRET is not configured")
	}

	timestamp, signature, ok := strings.Cut(strings.TrimSpace(header), ".")
	if !ok || timestamp == "" || signature == "" {
		return errors.New("missing or invalid X-Docuseal-Signature")
	}

	seconds, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return errors.New("invalid signature timestamp")
	}

	now := time.Now().Unix()
	maxAge := int64(s.cfg.DocuSealWebhookMaxAge)
	if seconds < now-maxAge {
		return errors.New("signature timestamp too old")
	}
	if seconds > now+maxAge {
		return errors.New("signature timestamp in future")
	}

	mac := hmac.New(sha256.New, []byte(s.cfg.DocuSealWebhookSecret))
	mac.Write([]byte(timestamp))
	mac.Write([]byte("."))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(expected), []byte(strings.ToLower(signature))) {
		return errors.New("signature mismatch")
	}

	return nil
}

func (s *Server) applyDocuSealWebhookEvent(ctx context.Context, eventType string, data map[string]any) error {
	switch eventType {
	case "submission.completed":
		docusealSubmissionID := stringID(data["id"])
		if err := s.store.UpdateDocuSealSubmissionStatus(ctx, docusealSubmissionID, "complete"); err != nil {
			return err
		}
		s.notifySubmissionCompleted(ctx, docusealSubmissionID, data)
		return nil
	case "submission.expired":
		return s.store.UpdateDocuSealSubmissionStatus(ctx, stringID(data["id"]), "failed")
	case "submission.archived":
		return s.store.CancelDocuSealSubmission(ctx, stringID(data["id"]))
	case "form.completed":
		if err := s.store.UpdateDocuSealSignerStatus(ctx, stringID(data["id"]), "complete"); err != nil {
			return err
		}
		if !strings.EqualFold(strings.TrimSpace(stringID(data["role"])), "Commander") {
			s.notifyCommanderSignatureRequested(ctx, docusealSubmissionIDFromWebhook(eventType, data), data)
		}
		return nil
	case "form.declined":
		if err := s.store.UpdateDocuSealSignerStatus(ctx, stringID(data["id"]), "canceled"); err != nil {
			return err
		}
		return s.store.CancelDocuSealSubmission(ctx, stringID(data["submission_id"]))
	case "template.created", "template.updated", "template.archived":
		s.syncDocuSealTemplates(ctx)
		return nil
	default:
		return nil
	}
}

func (s *Server) notifySubmissionCompleted(ctx context.Context, docusealSubmissionID string, data map[string]any) {
	submission, ok, err := s.store.NotificationSubmissionByDocuSealID(ctx, docusealSubmissionID)
	if err != nil {
		log.Printf("load completed submission notification context failed: %v", err)
		return
	}
	if !ok {
		return
	}

	recipients, err := s.store.SubmissionSignerRecipients(ctx, submission.ID)
	if err != nil {
		log.Printf("load completed submission recipients failed: %v", err)
		return
	}
	if len(recipients) == 0 {
		return
	}

	if err := s.notifications.Send(ctx, NotificationEvent{
		EventType:  "submission_completed",
		OccurredAt: time.Now().UTC(),
		Submission: submission,
		Recipients: recipients,
		DocuSeal:   data,
	}); err != nil {
		log.Printf("send completed submission notification failed: %v", err)
	}
}

func (s *Server) notifyCommanderSignatureRequested(ctx context.Context, docusealSubmissionID string, data map[string]any) {
	submission, ok, err := s.store.NotificationSubmissionByDocuSealID(ctx, docusealSubmissionID)
	if err != nil {
		log.Printf("load commander notification context failed: %v", err)
		return
	}
	if !ok || submission.Status != "pending" || !submission.RequiresCommander {
		return
	}

	recipients, err := s.store.CommanderRecipientsForUIC(ctx, submission.UIC)
	if err != nil {
		log.Printf("load commander notification recipients failed: %v", err)
		return
	}
	if len(recipients) == 0 {
		return
	}

	if err := s.notifications.Send(ctx, NotificationEvent{
		EventType:  "commander_signature_requested",
		OccurredAt: time.Now().UTC(),
		Submission: submission,
		Recipients: recipients,
		DocuSeal:   data,
	}); err != nil {
		log.Printf("send commander signature notification failed: %v", err)
	}
}

func docusealSubmissionIDFromWebhook(eventType string, data map[string]any) string {
	if strings.HasPrefix(eventType, "template.") {
		return ""
	}
	if id := stringID(data["submission_id"]); id != "" {
		return id
	}
	if submission, ok := data["submission"].(map[string]any); ok {
		return stringID(submission["id"])
	}
	return stringID(data["id"])
}

func (s *Server) handleDeleteSubmission(w http.ResponseWriter, r *http.Request, user User) {
	submissionID := strings.TrimSpace(r.PathValue("id"))
	if submissionID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "submission id is required"})
		return
	}

	submission, ok, err := s.store.SubmissionForUser(r.Context(), submissionID, user.ID)
	if err != nil {
		log.Printf("load submission for delete failed: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not load submission"})
		return
	}
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "submission not found"})
		return
	}
	if submission.Status == "complete" {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "completed submissions cannot be deleted"})
		return
	}

	if submission.DocuSealSubmissionID != "" {
		err := s.docuseal.ArchiveSubmission(r.Context(), submission.DocuSealSubmissionID)
		if errors.Is(err, ErrDocuSealNotConfigured) || errors.Is(err, ErrDocuSealNotFound) {
			err = nil
		}
		if err != nil {
			log.Printf("docuseal archive submission failed: %v", err)
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": "could not delete docuseal submission"})
			return
		}
	}

	if err := s.store.CancelSubmission(r.Context(), submission.ID); err != nil {
		log.Printf("cancel submission failed: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not delete submission"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "canceled"})
}

func (s *Server) handleUnitSubmissions(w http.ResponseWriter, r *http.Request, user User) {
	if !hasAny(user.Capabilities, "viewunit") {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "not authorized for unit submissions"})
		return
	}

	s.refreshPendingSubmissions(r.Context(), user)

	groups, err := s.store.UnitSubmissionGroups(r.Context(), user, r.URL.Query().Get("search"))
	if err != nil {
		log.Printf("load unit submissions failed: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not load unit submissions"})
		return
	}

	writeJSON(w, http.StatusOK, groups)
}

func (s *Server) refreshPendingSubmissions(ctx context.Context, user User) {
	ids, err := s.store.PendingDocuSealSubmissionIDs(ctx, user)
	if err != nil {
		log.Printf("load pending docuseal submission ids failed: %v", err)
		return
	}

	for _, id := range ids {
		status, err := s.docuseal.GetSubmissionStatus(ctx, id)
		if errors.Is(err, ErrDocuSealNotConfigured) {
			return
		}
		if errors.Is(err, ErrDocuSealNotFound) {
			if err := s.store.UpdateDocuSealSubmissionStatus(ctx, id, "canceled"); err != nil {
				log.Printf("mark missing docuseal submission failed: %v", err)
			}
			continue
		}
		if err != nil {
			log.Printf("refresh docuseal submission %s failed: %v", id, err)
			continue
		}
		if err := s.store.UpdateDocuSealSubmissionStatus(ctx, id, status.Status); err != nil {
			log.Printf("update docuseal submission %s status failed: %v", id, err)
		}
	}
}

func (s *Server) withAuth(next func(http.ResponseWriter, *http.Request, User)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(s.cfg.SessionCookieName)
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "not authenticated"})
			return
		}

		user, ok, err := s.store.UserForSession(r.Context(), cookie.Value)
		if err != nil {
			log.Printf("load session failed: %v", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not load session"})
			return
		}
		if !ok {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "not authenticated"})
			return
		}

		next(w, r, user)
	}
}

func (s *Server) withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == s.cfg.FrontendURL {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,DELETE,OPTIONS")
		}

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (s *Server) withHTTPS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.cfg.EnforceHTTPS {
			next.ServeHTTP(w, r)
			return
		}

		w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		if isHTTPSRequest(r) {
			next.ServeHTTP(w, r)
			return
		}

		writeJSON(w, http.StatusUpgradeRequired, map[string]string{"error": "https is required"})
	})
}

func isHTTPSRequest(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}

	forwardedProto := strings.ToLower(strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")))
	if forwardedProto == "https" {
		return true
	}

	forwardedSSL := strings.ToLower(strings.TrimSpace(r.Header.Get("X-Forwarded-Ssl")))
	return forwardedSSL == "on"
}

func (s *Server) setSessionCookie(w http.ResponseWriter, sessionID string) {
	http.SetCookie(w, &http.Cookie{
		Name:     s.cfg.SessionCookieName,
		Value:    sessionID,
		Path:     "/",
		HttpOnly: true,
		Secure:   s.cfg.SessionCookieSecure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int((8 * time.Hour).Seconds()),
	})
}

func (s *Server) enrichPayGradeFromRank(ctx context.Context, user *User) {
	if strings.TrimSpace(user.PayGrade) != "" || strings.TrimSpace(user.Rank) == "" {
		return
	}

	payGrade, ok, err := s.store.PayGradeForRank(ctx, user.Rank)
	if err != nil {
		log.Printf("derive pay grade from rank %q failed: %v", user.Rank, err)
		return
	}
	if ok {
		user.PayGrade = payGrade
	}
}

func formattedMilitaryName(claims LaunchClaims) string {
	last := strings.TrimSpace(claims.LastName)
	first := strings.TrimSpace(claims.FirstName)
	mi := strings.Trim(strings.TrimSpace(claims.MiddleInitial), ".")

	if last != "" && first != "" {
		name := last + ", " + first
		if mi != "" {
			name += " " + strings.ToUpper(mi[:1]) + "."
		}
		return name
	}

	return strings.TrimSpace(claims.FullName)
}

func docusealPrefillValues(user User) map[string]string {
	values := map[string]string{}
	nameRank := joinNonEmpty(" ", user.FullName, user.Rank)
	rankName := joinNonEmpty(" ", user.Rank, user.FullName)

	addPrefillValue(values, "Name", user.FullName)
	addPrefillValue(values, "Full Name", user.FullName)
	addPrefillValue(values, "Operator Name", user.FullName)
	addPrefillValue(values, "Commander's Name", user.FullName)
	addPrefillValue(values, "Commander Name", user.FullName)
	addPrefillValue(values, "Last name, First name MI", user.FullName)
	addPrefillValue(values, "Last Name, First Name MI", user.FullName)
	addPrefillValue(values, "Name & Rank", nameRank)
	addPrefillValue(values, "Name / Rank", nameRank)
	addPrefillValue(values, "Name Rank", nameRank)
	addPrefillValue(values, "Name and Rank", nameRank)
	addPrefillValue(values, "Rank & Name", rankName)
	addPrefillValue(values, "Rank / Name", rankName)
	addPrefillValue(values, "Rank Name", rankName)
	addPrefillValue(values, "Rank and Name", rankName)
	addPrefillValue(values, "First Name", user.FirstName)
	addPrefillValue(values, "Last Name", user.LastName)
	addPrefillValue(values, "Middle Initial", user.MiddleInitial)
	addPrefillValue(values, "Email", user.Email)
	addPrefillValue(values, "Email Address", user.Email)
	addPrefillValue(values, "DoD ID", user.DoDID)
	addPrefillValue(values, "DOD ID", user.DoDID)
	addPrefillValue(values, "Rank", user.Rank)
	addPrefillValue(values, "Grade", user.PayGrade)
	addPrefillValue(values, "Pay Grade", user.PayGrade)
	addPrefillValue(values, "UIC", user.UIC)
	return values
}

func addPrefillValue(values map[string]string, key string, value string) {
	value = strings.TrimSpace(value)
	if value != "" {
		values[key] = value
	}
}

func joinNonEmpty(separator string, values ...string) string {
	parts := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			parts = append(parts, value)
		}
	}
	return strings.Join(parts, separator)
}

func safePDFName(name string) string {
	name = strings.TrimSpace(strings.ToLower(name))
	if name == "" {
		name = "otasign-submission"
	}

	var builder strings.Builder
	lastDash := false
	for _, char := range name {
		if (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') {
			builder.WriteRune(char)
			lastDash = false
			continue
		}
		if !lastDash {
			builder.WriteByte('-')
			lastDash = true
		}
	}

	filename := strings.Trim(builder.String(), "-")
	if filename == "" {
		filename = "otasign-submission"
	}
	return filename + ".pdf"
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func randomID() string {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return time.Now().UTC().Format("20060102150405.000000000")
	}
	return base64.RawURLEncoding.EncodeToString(bytes)
}

func hasAny(values []string, wanted ...string) bool {
	lookup := map[string]bool{}
	for _, value := range values {
		lookup[strings.ToLower(value)] = true
	}
	for _, value := range wanted {
		if lookup[strings.ToLower(value)] {
			return true
		}
	}
	return false
}
