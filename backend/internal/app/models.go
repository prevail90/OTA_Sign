package app

import "time"

type LaunchClaims struct {
	MoodleUserID  string    `json:"moodle_user_id"`
	FullName      string    `json:"full_name"`
	FirstName     string    `json:"first_name,omitempty"`
	LastName      string    `json:"last_name,omitempty"`
	MiddleInitial string    `json:"middle_initial,omitempty"`
	Email         string    `json:"email"`
	DoDID         string    `json:"dod_id,omitempty"`
	Rank          string    `json:"rank,omitempty"`
	PayGrade      string    `json:"pay_grade,omitempty"`
	UIC           string    `json:"uic"`
	Roles         []string  `json:"roles"`
	Capabilities  []string  `json:"capabilities"`
	IssuedAt      time.Time `json:"issued_at"`
	ExpiresAt     time.Time `json:"expires_at"`
}

type User struct {
	ID            string   `json:"id"`
	MoodleUserID  string   `json:"moodle_user_id"`
	FullName      string   `json:"full_name"`
	FirstName     string   `json:"first_name,omitempty"`
	LastName      string   `json:"last_name,omitempty"`
	MiddleInitial string   `json:"middle_initial,omitempty"`
	Email         string   `json:"email"`
	DoDID         string   `json:"dod_id,omitempty"`
	Rank          string   `json:"rank,omitempty"`
	PayGrade      string   `json:"pay_grade,omitempty"`
	UIC           string   `json:"uic"`
	Roles         []string `json:"roles"`
	Capabilities  []string `json:"capabilities"`
}

type Template struct {
	ID                         string `json:"id"`
	Name                       string `json:"name"`
	DocuSealTemplateID         string `json:"docuseal_template_id"`
	SoldierRoleName            string `json:"soldier_role_name,omitempty"`
	RequiresCommanderSignature bool   `json:"requires_commander_signature"`
	CommanderRoleName          string `json:"commander_role_name,omitempty"`
}

type Submission struct {
	ID                   string     `json:"id"`
	TemplateID           string     `json:"template_id"`
	TemplateName         string     `json:"template_name"`
	SoldierUserID        string     `json:"soldier_user_id"`
	SoldierName          string     `json:"soldier_name"`
	SoldierDoDID         string     `json:"soldier_dod_id,omitempty"`
	UIC                  string     `json:"uic"`
	Status               string     `json:"status"`
	Current              bool       `json:"current"`
	RequiresCommander    bool       `json:"requires_commander"`
	WaitingOnCommander   bool       `json:"waiting_on_commander"`
	DocuSealSubmissionID string     `json:"docuseal_submission_id,omitempty"`
	SigningURL           string     `json:"signing_url,omitempty"`
	CreatedAt            time.Time  `json:"created_at"`
	CompletedAt          *time.Time `json:"completed_at,omitempty"`
}

type SoldierSubmissionGroup struct {
	SoldierUserID string       `json:"soldier_user_id"`
	SoldierName   string       `json:"soldier_name"`
	SoldierDoDID  string       `json:"soldier_dod_id,omitempty"`
	UIC           string       `json:"uic"`
	Submissions   []Submission `json:"submissions"`
}

type NotificationSubmission struct {
	ID                   string `json:"id"`
	TemplateID           string `json:"template_id"`
	TemplateName         string `json:"template_name"`
	SoldierName          string `json:"soldier_name"`
	SoldierEmail         string `json:"soldier_email"`
	SoldierDoDID         string `json:"soldier_dod_id,omitempty"`
	UIC                  string `json:"uic"`
	Status               string `json:"status"`
	DocuSealSubmissionID string `json:"docuseal_submission_id"`
}

type NotificationRecipient struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Role  string `json:"role"`
	UIC   string `json:"uic"`
}

type SubmissionDownload struct {
	SubmissionID          string
	TemplateName          string
	Status                string
	DocuSealSubmissionID  string
	DocuSealDocumentURL   string
	DocuSealDocumentName  string
	DocuSealDocumentBytes []byte
}
