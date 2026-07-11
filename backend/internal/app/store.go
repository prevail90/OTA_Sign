package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "github.com/lib/pq"
)

type Store struct {
	db *sql.DB
}

func NewStore(ctx context.Context, cfg Config) (*Store, error) {
	if cfg.DatabaseURL == "" {
		return nil, errors.New("DATABASE_URL is required")
	}

	db, err := sql.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, err
	}

	store := &Store{db: db}
	if err := store.Migrate(ctx, cfg.DatabaseMigrationsPath); err != nil {
		db.Close()
		return nil, err
	}

	return store, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) Check(ctx context.Context) error {
	return s.db.PingContext(ctx)
}

func (s *Store) Migrate(ctx context.Context, migrationsPath string) error {
	if strings.TrimSpace(migrationsPath) == "" {
		migrationsPath = "db/migrations"
	}

	entries, err := os.ReadDir(migrationsPath)
	if err != nil {
		return fmt.Errorf("read migrations: %w", err)
	}

	if _, err := s.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version TEXT PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)
	`); err != nil {
		return fmt.Errorf("ensure schema migrations table: %w", err)
	}

	var files []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		files = append(files, entry.Name())
	}
	sort.Strings(files)

	for _, file := range files {
		applied, err := s.migrationApplied(ctx, file)
		if err != nil {
			return err
		}
		if applied {
			continue
		}

		body, err := os.ReadFile(filepath.Join(migrationsPath, file))
		if err != nil {
			return fmt.Errorf("read migration %s: %w", file, err)
		}
		if err := s.applyMigration(ctx, file, string(body)); err != nil {
			return err
		}
	}

	return nil
}

func (s *Store) migrationApplied(ctx context.Context, version string) (bool, error) {
	var exists bool
	err := s.db.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM schema_migrations
			WHERE version = $1
		)
	`, version).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check migration %s: %w", version, err)
	}
	return exists, nil
}

func (s *Store) applyMigration(ctx context.Context, version string, statement string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, statement); err != nil {
		return fmt.Errorf("apply migration %s: %w", version, err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO schema_migrations (version)
		VALUES ($1)
	`, version); err != nil {
		return fmt.Errorf("record migration %s: %w", version, err)
	}

	return tx.Commit()
}

func (s *Store) SaveSession(ctx context.Context, sessionID string, user User) error {
	roles, err := json.Marshal(user.Roles)
	if err != nil {
		return err
	}

	capabilities, err := json.Marshal(user.Capabilities)
	if err != nil {
		return err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var userID string
	if err := tx.QueryRowContext(ctx, `
		INSERT INTO users (
			moodle_user_id,
			full_name,
			first_name,
			last_name,
			middle_initial,
			email,
			dod_id,
			rank,
			pay_grade
		)
		VALUES ($1, $2, nullif($3, ''), nullif($4, ''), nullif($5, ''), $6, nullif($7, ''), nullif($8, ''), nullif($9, ''))
		ON CONFLICT (moodle_user_id) DO UPDATE SET
			full_name = EXCLUDED.full_name,
			first_name = EXCLUDED.first_name,
			last_name = EXCLUDED.last_name,
			middle_initial = EXCLUDED.middle_initial,
			email = EXCLUDED.email,
			dod_id = EXCLUDED.dod_id,
			rank = EXCLUDED.rank,
			pay_grade = EXCLUDED.pay_grade,
			updated_at = now()
		RETURNING id::text
	`, user.MoodleUserID, user.FullName, user.FirstName, user.LastName, user.MiddleInitial, user.Email, user.DoDID, user.Rank, user.PayGrade).Scan(&userID); err != nil {
		return err
	}

	var unitID string
	if err := tx.QueryRowContext(ctx, `
		INSERT INTO units (uic)
		VALUES ($1)
		ON CONFLICT (uic) DO UPDATE SET updated_at = now()
		RETURNING id::text
	`, user.UIC).Scan(&unitID); err != nil {
		return err
	}

	for _, role := range append(prefixedValues("role:", user.Roles), prefixedValues("capability:", user.Capabilities)...) {
		if strings.TrimSpace(role) == "" {
			continue
		}

		if _, err := tx.ExecContext(ctx, `
			INSERT INTO user_unit_roles (user_id, unit_id, role)
			VALUES ($1::uuid, $2::uuid, $3)
			ON CONFLICT (user_id, unit_id, role) DO UPDATE SET updated_at = now()
		`, userID, unitID, role); err != nil {
			return err
		}
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO user_unit_roles (user_id, unit_id, role)
		VALUES ($1::uuid, $2::uuid, 'member')
		ON CONFLICT (user_id, unit_id, role) DO UPDATE SET updated_at = now()
	`, userID, unitID); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO sessions (id, user_id, roles, capabilities, expires_at)
		VALUES ($1, $2::uuid, $3::jsonb, $4::jsonb, $5)
		ON CONFLICT (id) DO UPDATE SET
			user_id = EXCLUDED.user_id,
			roles = EXCLUDED.roles,
			capabilities = EXCLUDED.capabilities,
			expires_at = EXCLUDED.expires_at
	`, sessionID, userID, string(roles), string(capabilities), time.Now().UTC().Add(8*time.Hour)); err != nil {
		return err
	}

	return tx.Commit()
}

func (s *Store) UserForSession(ctx context.Context, sessionID string) (User, bool, error) {
	var user User
	var rolesJSON, capabilitiesJSON []byte

	err := s.db.QueryRowContext(ctx, `
		SELECT
			u.id::text,
			u.moodle_user_id,
			u.full_name,
			coalesce(u.first_name, ''),
			coalesce(u.last_name, ''),
			coalesce(u.middle_initial, ''),
			u.email,
			coalesce(u.dod_id, ''),
			coalesce(u.rank, ''),
			coalesce(u.pay_grade, ''),
			un.uic,
			s.roles,
			s.capabilities
		FROM sessions s
		JOIN users u ON u.id = s.user_id
		JOIN user_unit_roles uur ON uur.user_id = u.id
		JOIN units un ON un.id = uur.unit_id
		WHERE s.id = $1
		  AND s.expires_at > now()
		ORDER BY uur.updated_at DESC
		LIMIT 1
	`, sessionID).Scan(
		&user.ID,
		&user.MoodleUserID,
		&user.FullName,
		&user.FirstName,
		&user.LastName,
		&user.MiddleInitial,
		&user.Email,
		&user.DoDID,
		&user.Rank,
		&user.PayGrade,
		&user.UIC,
		&rolesJSON,
		&capabilitiesJSON,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, false, nil
	}
	if err != nil {
		return User{}, false, err
	}

	if err := json.Unmarshal(rolesJSON, &user.Roles); err != nil {
		return User{}, false, err
	}
	if err := json.Unmarshal(capabilitiesJSON, &user.Capabilities); err != nil {
		return User{}, false, err
	}

	return user, true, nil
}

func (s *Store) TemplatesForUser(ctx context.Context, user User) ([]Template, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT
			id::text,
			name,
			docuseal_template_id,
			soldier_role_name,
			requires_commander_signature,
			coalesce(commander_role_name, '')
		FROM templates
		WHERE active = true
		ORDER BY name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var templates []Template
	for rows.Next() {
		var template Template
		if err := rows.Scan(
			&template.ID,
			&template.Name,
			&template.DocuSealTemplateID,
			&template.SoldierRoleName,
			&template.RequiresCommanderSignature,
			&template.CommanderRoleName,
		); err != nil {
			return nil, err
		}
		templates = append(templates, template)
	}

	return templates, rows.Err()
}

func (s *Store) SyncDocuSealTemplates(ctx context.Context, docusealTemplates []DocuSealTemplate) error {
	if len(docusealTemplates) == 0 {
		return nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, docusealTemplate := range docusealTemplates {
		if docusealTemplate.ID == "" || strings.TrimSpace(docusealTemplate.Name) == "" {
			continue
		}

		soldierRole, commanderRole := docusealTemplateRoles(docusealTemplate.Submitters)
		requiresCommander := commanderRole != ""
		active := !docusealTemplate.Archived

		if _, err := tx.ExecContext(ctx, `
			INSERT INTO templates (
				docuseal_template_id,
				docuseal_template_slug,
				soldier_role_name,
				name,
				active,
				requires_commander_signature,
				commander_role_name
			)
			VALUES ($1, nullif($2, ''), $3, $4, $5, $6, nullif($7, ''))
			ON CONFLICT (docuseal_template_id) DO UPDATE SET
				docuseal_template_slug = EXCLUDED.docuseal_template_slug,
				soldier_role_name = EXCLUDED.soldier_role_name,
				name = EXCLUDED.name,
				active = EXCLUDED.active,
				requires_commander_signature = EXCLUDED.requires_commander_signature,
				commander_role_name = EXCLUDED.commander_role_name,
				updated_at = now()
		`, docusealTemplate.ID, docusealTemplate.Slug, soldierRole, docusealTemplate.Name, active, requiresCommander, commanderRole); err != nil {
			return err
		}
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE templates
		SET active = false,
			updated_at = now()
		WHERE docuseal_template_id LIKE 'docuseal-template-%'
	`); err != nil {
		return err
	}

	return tx.Commit()
}

func (s *Store) TemplateByID(ctx context.Context, templateID string) (Template, bool, error) {
	var template Template

	err := s.db.QueryRowContext(ctx, `
		SELECT
			id::text,
			name,
			docuseal_template_id,
			soldier_role_name,
			requires_commander_signature,
			coalesce(commander_role_name, '')
		FROM templates
		WHERE id = $1::uuid
		  AND active = true
	`, templateID).Scan(
		&template.ID,
		&template.Name,
		&template.DocuSealTemplateID,
		&template.SoldierRoleName,
		&template.RequiresCommanderSignature,
		&template.CommanderRoleName,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Template{}, false, nil
	}
	if err != nil {
		return Template{}, false, err
	}

	return template, true, nil
}

func docusealTemplateRoles(roles []string) (string, string) {
	soldierRole := ""
	commanderRole := ""

	for _, role := range roles {
		switch strings.ToLower(strings.TrimSpace(role)) {
		case "soldier":
			soldierRole = role
		case "commander":
			commanderRole = role
		}
	}

	if soldierRole == "" && len(roles) > 0 {
		soldierRole = roles[0]
	}
	if soldierRole == "" {
		soldierRole = "Soldier"
	}

	return soldierRole, commanderRole
}

func (s *Store) CreateSubmission(ctx context.Context, user User, template Template, docusealResult DocuSealSubmissionResult) (Submission, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Submission{}, err
	}
	defer tx.Rollback()

	var unitID string
	if err := tx.QueryRowContext(ctx, `
		SELECT id::text
		FROM units
		WHERE uic = $1
	`, user.UIC).Scan(&unitID); err != nil {
		return Submission{}, err
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE submissions
		SET current = false,
			updated_at = now()
		WHERE soldier_user_id = $1::uuid
		  AND template_id = $2::uuid
		  AND current = true
	`, user.ID, template.ID); err != nil {
		return Submission{}, err
	}

	var submission Submission
	if err := tx.QueryRowContext(ctx, `
		INSERT INTO submissions (
			template_id,
			soldier_user_id,
			unit_id,
			docuseal_submission_id,
			status,
			current
		)
		VALUES ($1::uuid, $2::uuid, $3::uuid, $4, 'pending', true)
		RETURNING id::text, created_at
	`, template.ID, user.ID, unitID, docusealResult.SubmissionID).Scan(&submission.ID, &submission.CreatedAt); err != nil {
		return Submission{}, err
	}

	soldierSubmitter, ok := docusealResult.SubmitterForRole(template.SoldierRoleName)
	if !ok {
		soldierSubmitter = docusealResult.Submitters[0]
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO submission_signers (
			submission_id,
			user_id,
			role,
			name,
			email,
			docuseal_submitter_id,
			signing_url,
			status
		)
		VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7, 'pending')
	`, submission.ID, user.ID, template.SoldierRoleName, user.FullName, user.Email, soldierSubmitter.SubmitterID, soldierSubmitter.SigningURL); err != nil {
		return Submission{}, err
	}

	if template.RequiresCommanderSignature && template.CommanderRoleName != "" {
		commanderSubmitter, ok := docusealResult.SubmitterForRole(template.CommanderRoleName)
		if ok {
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO submission_signers (
					submission_id,
					user_id,
					role,
					name,
					email,
					docuseal_submitter_id,
					signing_url,
					status
				)
				VALUES ($1::uuid, null, $2, 'Pending Commander', '', $3, $4, 'pending')
			`, submission.ID, template.CommanderRoleName, commanderSubmitter.SubmitterID, commanderSubmitter.SigningURL); err != nil {
				return Submission{}, err
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return Submission{}, err
	}

	submission.TemplateID = template.ID
	submission.TemplateName = template.Name
	submission.SoldierUserID = user.ID
	submission.SoldierName = user.FullName
	submission.SoldierDoDID = user.DoDID
	submission.UIC = user.UIC
	submission.Status = "pending"
	submission.Current = true
	submission.RequiresCommander = template.RequiresCommanderSignature
	submission.WaitingOnCommander = template.RequiresCommanderSignature && template.CommanderRoleName != ""
	submission.DocuSealSubmissionID = docusealResult.SubmissionID
	submission.SigningURL = soldierSubmitter.SigningURL

	return submission, nil
}

func (s *Store) MySubmissions(ctx context.Context, user User) ([]Submission, error) {
	templates, err := s.TemplatesForUser(ctx, user)
	if err != nil {
		return nil, err
	}

	submissions, err := s.submissionsForUser(ctx, user.ID)
	if err != nil {
		return nil, err
	}

	return withMissingSubmissions(templates, submissions, user), nil
}

func (s *Store) PendingDocuSealSubmissionIDs(ctx context.Context, user User) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT DISTINCT s.docuseal_submission_id
		FROM submissions s
		JOIN units un ON un.id = s.unit_id
		WHERE s.current = true
		  AND s.status = 'pending'
		  AND s.docuseal_submission_id IS NOT NULL
		  AND (
			s.soldier_user_id = $1::uuid
			OR un.uic = $2
		  )
	`, user.ID, user.UIC)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}

	return ids, rows.Err()
}

func (s *Store) UpdateDocuSealSubmissionStatus(ctx context.Context, docusealSubmissionID string, status string) error {
	current := status == "pending"
	completedAtSQL := "completed_at"
	if status == "complete" {
		completedAtSQL = "now()"
	}

	_, err := s.db.ExecContext(ctx, fmt.Sprintf(`
		UPDATE submissions
		SET status = $1,
			current = $2,
			completed_at = %s,
			updated_at = now()
		WHERE docuseal_submission_id = $3
	`, completedAtSQL), status, current, docusealSubmissionID)
	return err
}

func (s *Store) UpdateDocuSealSignerStatus(ctx context.Context, docusealSubmitterID string, status string) error {
	if strings.TrimSpace(docusealSubmitterID) == "" {
		return nil
	}

	_, err := s.db.ExecContext(ctx, `
		UPDATE submission_signers
		SET status = $1,
			updated_at = now()
		WHERE docuseal_submitter_id = $2
	`, status, docusealSubmitterID)
	return err
}

func (s *Store) CancelDocuSealSubmission(ctx context.Context, docusealSubmissionID string) error {
	if strings.TrimSpace(docusealSubmissionID) == "" {
		return nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `
		UPDATE submissions
		SET status = 'canceled',
			current = false,
			updated_at = now()
		WHERE docuseal_submission_id = $1
	`, docusealSubmissionID); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE submission_signers ss
		SET status = 'canceled',
			updated_at = now()
		FROM submissions s
		WHERE ss.submission_id = s.id
		  AND s.docuseal_submission_id = $1
		  AND ss.status <> 'complete'
	`, docusealSubmissionID); err != nil {
		return err
	}

	return tx.Commit()
}

func (s *Store) RecordDocuSealWebhookEvent(ctx context.Context, eventID string, eventType string, docusealSubmissionID string, payload []byte) error {
	eventID = strings.TrimSpace(eventID)
	docusealSubmissionID = strings.TrimSpace(docusealSubmissionID)

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO docuseal_webhook_events (
			event_id,
			event_type,
			docuseal_submission_id,
			payload,
			processed_at
		)
		VALUES (nullif($1, ''), $2, nullif($3, ''), $4::jsonb, now())
		ON CONFLICT (event_id) DO UPDATE SET
			event_type = EXCLUDED.event_type,
			docuseal_submission_id = EXCLUDED.docuseal_submission_id,
			payload = EXCLUDED.payload,
			processed_at = now()
	`, eventID, eventType, docusealSubmissionID, string(payload))
	return err
}

func (s *Store) NotificationSubmissionByDocuSealID(ctx context.Context, docusealSubmissionID string) (NotificationSubmission, bool, error) {
	var submission NotificationSubmission

	err := s.db.QueryRowContext(ctx, `
		SELECT
			s.id::text,
			t.id::text,
			t.name,
			u.full_name,
			u.email,
			coalesce(u.dod_id, ''),
			un.uic,
			s.status,
			t.requires_commander_signature,
			coalesce(s.docuseal_submission_id, '')
		FROM submissions s
		JOIN templates t ON t.id = s.template_id
		JOIN users u ON u.id = s.soldier_user_id
		JOIN units un ON un.id = s.unit_id
		WHERE s.docuseal_submission_id = $1
	`, docusealSubmissionID).Scan(
		&submission.ID,
		&submission.TemplateID,
		&submission.TemplateName,
		&submission.SoldierName,
		&submission.SoldierEmail,
		&submission.SoldierDoDID,
		&submission.UIC,
		&submission.Status,
		&submission.RequiresCommander,
		&submission.DocuSealSubmissionID,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return NotificationSubmission{}, false, nil
	}
	if err != nil {
		return NotificationSubmission{}, false, err
	}

	return submission, true, nil
}

func (s *Store) SubmissionSignerRecipients(ctx context.Context, submissionID string) ([]NotificationRecipient, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT DISTINCT
			ss.name,
			ss.email,
			ss.role,
			un.uic
		FROM submission_signers ss
		JOIN submissions s ON s.id = ss.submission_id
		JOIN units un ON un.id = s.unit_id
		WHERE ss.submission_id = $1::uuid
		  AND ss.email <> ''
		ORDER BY ss.role, ss.email
	`, submissionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var recipients []NotificationRecipient
	for rows.Next() {
		var recipient NotificationRecipient
		if err := rows.Scan(&recipient.Name, &recipient.Email, &recipient.Role, &recipient.UIC); err != nil {
			return nil, err
		}
		recipients = append(recipients, recipient)
	}

	return recipients, rows.Err()
}

func (s *Store) CommanderRecipientsForUIC(ctx context.Context, uic string) ([]NotificationRecipient, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT DISTINCT
			u.full_name,
			u.email,
			un.uic
		FROM users u
		JOIN user_unit_roles uur ON uur.user_id = u.id
		JOIN units un ON un.id = uur.unit_id
		WHERE un.uic = $1
		  AND lower(uur.role) = 'capability:signascommander'
		ORDER BY u.full_name, u.email
	`, uic)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var recipients []NotificationRecipient
	for rows.Next() {
		var recipient NotificationRecipient
		if err := rows.Scan(&recipient.Name, &recipient.Email, &recipient.UIC); err != nil {
			return nil, err
		}
		recipient.Role = "Commander"
		recipients = append(recipients, recipient)
	}

	return recipients, rows.Err()
}

func (s *Store) PayGradeForRank(ctx context.Context, rank string) (string, bool, error) {
	rank = strings.TrimSpace(rank)
	if rank == "" {
		return "", false, nil
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT DISTINCT pay_grade
		FROM dod_paygrades
		WHERE upper(rank_abbreviation) = upper($1)
		ORDER BY pay_grade
	`, rank)
	if err != nil {
		return "", false, err
	}
	defer rows.Close()

	var payGrades []string
	for rows.Next() {
		var payGrade string
		if err := rows.Scan(&payGrade); err != nil {
			return "", false, err
		}
		payGrades = append(payGrades, payGrade)
	}
	if err := rows.Err(); err != nil {
		return "", false, err
	}
	if len(payGrades) == 0 {
		return "", false, nil
	}
	if len(payGrades) > 1 {
		return "", false, fmt.Errorf("rank %q maps to multiple pay grades: %s", rank, strings.Join(payGrades, ", "))
	}

	return payGrades[0], true, nil
}

func (s *Store) SubmissionForUser(ctx context.Context, submissionID string, userID string) (Submission, bool, error) {
	var submission Submission

	err := s.db.QueryRowContext(ctx, `
		SELECT
			id::text,
			status,
			current,
			coalesce(docuseal_submission_id, '')
		FROM submissions
		WHERE id = $1::uuid
		  AND soldier_user_id = $2::uuid
	`, submissionID, userID).Scan(
		&submission.ID,
		&submission.Status,
		&submission.Current,
		&submission.DocuSealSubmissionID,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Submission{}, false, nil
	}
	if err != nil {
		return Submission{}, false, err
	}

	return submission, true, nil
}

func (s *Store) SubmissionDownloadForUser(ctx context.Context, submissionID string, user User) (SubmissionDownload, bool, error) {
	var download SubmissionDownload

	err := s.db.QueryRowContext(ctx, `
		SELECT
			s.id::text,
			t.name,
			s.status,
			coalesce(s.docuseal_submission_id, '')
		FROM submissions s
		JOIN templates t ON t.id = s.template_id
		JOIN units un ON un.id = s.unit_id
		WHERE s.id = $1::uuid
		  AND (
			s.soldier_user_id = $2::uuid
			OR (
				un.uic = $3
				AND $4
			)
		  )
	`, submissionID, user.ID, user.UIC, hasAny(user.Capabilities, "viewunit")).Scan(
		&download.SubmissionID,
		&download.TemplateName,
		&download.Status,
		&download.DocuSealSubmissionID,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return SubmissionDownload{}, false, nil
	}
	if err != nil {
		return SubmissionDownload{}, false, err
	}

	return download, true, nil
}

func (s *Store) CommanderSignerForSubmission(ctx context.Context, submissionID string, user User) (Submission, string, bool, error) {
	var submission Submission
	var docusealSubmitterID string

	err := s.db.QueryRowContext(ctx, `
		SELECT
			s.id::text,
			t.id::text,
			t.name,
			u.id::text,
			u.full_name,
			coalesce(u.dod_id, ''),
			un.uic,
			s.status,
			s.current,
			t.requires_commander_signature,
			coalesce(s.docuseal_submission_id, ''),
			coalesce(ss.docuseal_submitter_id, '')
		FROM submissions s
		JOIN templates t ON t.id = s.template_id
		JOIN users u ON u.id = s.soldier_user_id
		JOIN units un ON un.id = s.unit_id
		JOIN submission_signers ss ON ss.submission_id = s.id
		WHERE s.id = $1::uuid
		  AND un.uic = $2
		  AND s.status = 'pending'
		  AND s.current = true
		  AND t.requires_commander_signature = true
		  AND ss.status = 'pending'
		  AND lower(ss.role) = lower(coalesce(t.commander_role_name, 'Commander'))
		LIMIT 1
	`, submissionID, user.UIC).Scan(
		&submission.ID,
		&submission.TemplateID,
		&submission.TemplateName,
		&submission.SoldierUserID,
		&submission.SoldierName,
		&submission.SoldierDoDID,
		&submission.UIC,
		&submission.Status,
		&submission.Current,
		&submission.RequiresCommander,
		&submission.DocuSealSubmissionID,
		&docusealSubmitterID,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Submission{}, "", false, nil
	}
	if err != nil {
		return Submission{}, "", false, err
	}

	return submission, docusealSubmitterID, true, nil
}

func (s *Store) AssignCommanderSigner(ctx context.Context, submissionID string, user User, docusealSubmitterID string, signingURL string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE submission_signers
		SET user_id = $1::uuid,
			name = $2,
			email = $3,
			signing_url = $4,
			updated_at = now()
		WHERE submission_id = $5::uuid
		  AND docuseal_submitter_id = $6
		  AND status = 'pending'
	`, user.ID, user.FullName, user.Email, signingURL, submissionID, docusealSubmitterID)
	return err
}

func (s *Store) CancelSubmission(ctx context.Context, submissionID string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `
		UPDATE submissions
		SET status = 'canceled',
			current = false,
			updated_at = now()
		WHERE id = $1::uuid
	`, submissionID); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE submission_signers
		SET status = 'canceled',
			updated_at = now()
		WHERE submission_id = $1::uuid
		  AND status <> 'complete'
	`, submissionID); err != nil {
		return err
	}

	return tx.Commit()
}

func (s *Store) UnitSubmissionGroups(ctx context.Context, user User, search string) ([]SoldierSubmissionGroup, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT
			u.id::text,
			u.full_name,
			coalesce(u.dod_id, '')
		FROM users u
		JOIN user_unit_roles uur ON uur.user_id = u.id
		JOIN units un ON un.id = uur.unit_id
		WHERE un.uic = $1
		GROUP BY u.id, u.full_name, u.dod_id
		ORDER BY u.full_name
	`, user.UIC)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	templates, err := s.TemplatesForUser(ctx, user)
	if err != nil {
		return nil, err
	}

	var groups []SoldierSubmissionGroup
	for rows.Next() {
		groupUser := User{UIC: user.UIC}
		if err := rows.Scan(&groupUser.ID, &groupUser.FullName, &groupUser.DoDID); err != nil {
			return nil, err
		}

		submissions, err := s.submissionsForUser(ctx, groupUser.ID)
		if err != nil {
			return nil, err
		}

		visibleSubmissions := withoutCanceledSubmissions(submissions)
		groups = append(groups, SoldierSubmissionGroup{
			SoldierUserID: groupUser.ID,
			SoldierName:   groupUser.FullName,
			SoldierDoDID:  groupUser.DoDID,
			UIC:           groupUser.UIC,
			Submissions:   withMissingUnitSubmissions(templates, visibleSubmissions, submissions, groupUser),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return filterUnitGroups(groups, search), nil
}

func (s *Store) submissionsForUser(ctx context.Context, userID string) ([]Submission, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT
			s.id::text,
			t.id::text,
			t.name,
			u.id::text,
			u.full_name,
			coalesce(u.dod_id, ''),
			un.uic,
			s.status,
			s.current,
			t.requires_commander_signature,
			EXISTS (
				SELECT 1
				FROM submission_signers commander_signer
				WHERE commander_signer.submission_id = s.id
				  AND commander_signer.status = 'pending'
				  AND lower(commander_signer.role) = lower(coalesce(t.commander_role_name, 'Commander'))
			),
			coalesce(s.docuseal_submission_id, ''),
			coalesce((
				SELECT ss.signing_url
				FROM submission_signers ss
				WHERE ss.submission_id = s.id
				  AND ss.user_id = u.id
				ORDER BY ss.created_at
				LIMIT 1
			), ''),
			s.created_at,
			s.completed_at
		FROM submissions s
		JOIN templates t ON t.id = s.template_id
		JOIN users u ON u.id = s.soldier_user_id
		JOIN units un ON un.id = s.unit_id
		WHERE s.soldier_user_id = $1::uuid
		ORDER BY s.created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var submissions []Submission
	for rows.Next() {
		var submission Submission
		if err := rows.Scan(
			&submission.ID,
			&submission.TemplateID,
			&submission.TemplateName,
			&submission.SoldierUserID,
			&submission.SoldierName,
			&submission.SoldierDoDID,
			&submission.UIC,
			&submission.Status,
			&submission.Current,
			&submission.RequiresCommander,
			&submission.WaitingOnCommander,
			&submission.DocuSealSubmissionID,
			&submission.SigningURL,
			&submission.CreatedAt,
			&submission.CompletedAt,
		); err != nil {
			return nil, err
		}
		submissions = append(submissions, submission)
	}

	return submissions, rows.Err()
}

func prefixedValues(prefix string, values []string) []string {
	next := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			next = append(next, prefix+value)
		}
	}
	return next
}

func withMissingSubmissions(templates []Template, submissions []Submission, user User) []Submission {
	now := time.Now().UTC()
	satisfiedByTemplate := map[string]bool{}
	for _, submission := range submissions {
		if submission.Current || submission.Status == "complete" {
			satisfiedByTemplate[submission.TemplateID] = true
		}
	}

	for _, template := range templates {
		if satisfiedByTemplate[template.ID] {
			continue
		}

		submissions = append(submissions, Submission{
			ID:                 "missing-" + template.ID + "-" + user.ID,
			TemplateID:         template.ID,
			TemplateName:       template.Name,
			SoldierUserID:      user.ID,
			SoldierName:        user.FullName,
			SoldierDoDID:       user.DoDID,
			UIC:                user.UIC,
			Status:             "missing",
			Current:            true,
			RequiresCommander:  template.RequiresCommanderSignature,
			WaitingOnCommander: false,
			CreatedAt:          now,
		})
	}

	sortSubmissions(submissions)
	return submissions
}

func withMissingUnitSubmissions(templates []Template, visibleSubmissions []Submission, allSubmissions []Submission, user User) []Submission {
	satisfiedByTemplate := map[string]bool{}
	for _, submission := range allSubmissions {
		if submission.Current || submission.Status == "pending" || submission.Status == "complete" {
			satisfiedByTemplate[submission.TemplateID] = true
		}
	}

	return withMissingSubmissionsExcept(templates, visibleSubmissions, satisfiedByTemplate, user)
}

func withMissingSubmissionsExcept(templates []Template, submissions []Submission, skipMissingByTemplate map[string]bool, user User) []Submission {
	now := time.Now().UTC()
	for _, template := range templates {
		if skipMissingByTemplate[template.ID] {
			continue
		}

		submissions = append(submissions, Submission{
			ID:                 "missing-" + template.ID + "-" + user.ID,
			TemplateID:         template.ID,
			TemplateName:       template.Name,
			SoldierUserID:      user.ID,
			SoldierName:        user.FullName,
			SoldierDoDID:       user.DoDID,
			UIC:                user.UIC,
			Status:             "missing",
			Current:            true,
			RequiresCommander:  template.RequiresCommanderSignature,
			WaitingOnCommander: false,
			CreatedAt:          now,
		})
	}

	sortSubmissions(submissions)
	return submissions
}

func withoutCanceledSubmissions(submissions []Submission) []Submission {
	next := make([]Submission, 0, len(submissions))
	for _, submission := range submissions {
		if submission.Status == "canceled" {
			continue
		}
		next = append(next, submission)
	}
	return next
}

func sortSubmissions(submissions []Submission) {
	sort.SliceStable(submissions, func(i, j int) bool {
		if submissions[i].Status == "missing" && submissions[j].Status != "missing" {
			return false
		}
		if submissions[i].Status != "missing" && submissions[j].Status == "missing" {
			return true
		}
		return submissions[i].TemplateName < submissions[j].TemplateName
	})
}

func filterUnitGroups(groups []SoldierSubmissionGroup, search string) []SoldierSubmissionGroup {
	search = strings.ToLower(strings.TrimSpace(search))
	if search == "" {
		return groups
	}

	filtered := make([]SoldierSubmissionGroup, 0, len(groups))
	for _, group := range groups {
		groupMatches := strings.Contains(strings.ToLower(group.SoldierName), search) ||
			strings.Contains(strings.ToLower(group.SoldierDoDID), search)

		next := group
		next.Submissions = nil

		for _, submission := range group.Submissions {
			formMatches := strings.Contains(strings.ToLower(submission.TemplateName), search)
			if groupMatches || formMatches {
				next.Submissions = append(next.Submissions, submission)
			}
		}

		if len(next.Submissions) > 0 {
			filtered = append(filtered, next)
		}
	}

	return filtered
}
