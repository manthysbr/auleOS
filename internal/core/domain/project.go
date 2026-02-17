package domain

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"
)

// ProjectID uniquely identifies a project
type ProjectID string

// Project groups conversations and artifacts under a single workspace context
type Project struct {
	ID          ProjectID `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// ArtifactID uniquely identifies an artifact
type ArtifactID string

// ArtifactType classifies the artifact content
type ArtifactType string

const (
	ArtifactTypeImage    ArtifactType = "image"
	ArtifactTypeText     ArtifactType = "text"
	ArtifactTypeDocument ArtifactType = "document"
	ArtifactTypeAudio    ArtifactType = "audio"
	ArtifactTypeVideo    ArtifactType = "video"
	ArtifactTypeOther    ArtifactType = "other"
)

// Artifact represents a generated file/output (image, text, doc, etc.)
type Artifact struct {
	ID             ArtifactID      `json:"id"`
	ProjectID      *ProjectID      `json:"project_id,omitempty"`
	JobID          *JobID          `json:"job_id,omitempty"`
	ConversationID *ConversationID `json:"conversation_id,omitempty"`
	Type           ArtifactType    `json:"type"`
	Name           string          `json:"name"`
	FilePath       string          `json:"file_path"`
	MimeType       string          `json:"mime_type"`
	SizeBytes      int64           `json:"size_bytes"`
	Prompt         string          `json:"prompt,omitempty"`
	CreatedAt      time.Time       `json:"created_at"`
}

var (
	ErrProjectNotFound  = errors.New("project not found")
	ErrArtifactNotFound = errors.New("artifact not found")
)

// NewProjectID generates a compact random project ID (proj-<12 hex>)
func NewProjectID() ProjectID {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return ProjectID("proj-" + hex.EncodeToString(b))
}

// NewArtifactID generates a compact random artifact ID (art-<12 hex>)
func NewArtifactID() ArtifactID {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return ArtifactID("art-" + hex.EncodeToString(b))
}
