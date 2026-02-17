package kernel

import (
	"context"
	"time"

	"github.com/manthysbr/auleOS/internal/core/domain"
)

// --- StrictServerInterface implementations for Projects ---

// ListProjects implements StrictServerInterface.
func (s *Server) ListProjects(ctx context.Context, _ ListProjectsRequestObject) (ListProjectsResponseObject, error) {
	projects, err := s.repo.ListProjects(ctx)
	if err != nil {
		s.logger.Error("failed to list projects", "error", err)
		return ListProjects200JSONResponse{}, nil
	}

	result := make(ListProjects200JSONResponse, len(projects))
	for i, p := range projects {
		result[i] = domainProjectToAPI(p)
	}
	return result, nil
}

// CreateProject implements StrictServerInterface.
func (s *Server) CreateProject(ctx context.Context, request CreateProjectRequestObject) (CreateProjectResponseObject, error) {
	now := time.Now()
	proj := domain.Project{
		ID:        domain.NewProjectID(),
		Name:      request.Body.Name,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if request.Body.Description != nil {
		proj.Description = *request.Body.Description
	}

	if err := s.repo.CreateProject(ctx, proj); err != nil {
		s.logger.Error("failed to create project", "error", err)
		return nil, err
	}

	return CreateProject201JSONResponse(domainProjectToAPI(proj)), nil
}

// GetProject implements StrictServerInterface.
func (s *Server) GetProject(ctx context.Context, request GetProjectRequestObject) (GetProjectResponseObject, error) {
	proj, err := s.repo.GetProject(ctx, domain.ProjectID(request.Id))
	if err != nil {
		if err == domain.ErrProjectNotFound {
			msg := "project not found"
			return GetProject404JSONResponse{Error: &msg}, nil
		}
		s.logger.Error("failed to get project", "error", err)
		return nil, err
	}
	return GetProject200JSONResponse(domainProjectToAPI(proj)), nil
}

// UpdateProject implements StrictServerInterface.
func (s *Server) UpdateProject(ctx context.Context, request UpdateProjectRequestObject) (UpdateProjectResponseObject, error) {
	id := domain.ProjectID(request.Id)

	proj, err := s.repo.GetProject(ctx, id)
	if err != nil {
		if err == domain.ErrProjectNotFound {
			msg := "project not found"
			return UpdateProject404JSONResponse{Error: &msg}, nil
		}
		return nil, err
	}

	if request.Body.Name != nil {
		proj.Name = *request.Body.Name
	}
	if request.Body.Description != nil {
		proj.Description = *request.Body.Description
	}
	proj.UpdatedAt = time.Now()

	if err := s.repo.UpdateProject(ctx, proj); err != nil {
		s.logger.Error("failed to update project", "error", err)
		return nil, err
	}

	return UpdateProject200JSONResponse(domainProjectToAPI(proj)), nil
}

// DeleteProject implements StrictServerInterface.
func (s *Server) DeleteProject(ctx context.Context, request DeleteProjectRequestObject) (DeleteProjectResponseObject, error) {
	if err := s.repo.DeleteProject(ctx, domain.ProjectID(request.Id)); err != nil {
		if err == domain.ErrProjectNotFound {
			return DeleteProject404Response{}, nil
		}
		s.logger.Error("failed to delete project", "error", err)
		return nil, err
	}
	return DeleteProject204Response{}, nil
}

// ListProjectConversations implements StrictServerInterface.
func (s *Server) ListProjectConversations(ctx context.Context, request ListProjectConversationsRequestObject) (ListProjectConversationsResponseObject, error) {
	convs, err := s.repo.ListProjectConversations(ctx, domain.ProjectID(request.Id))
	if err != nil {
		s.logger.Error("failed to list project conversations", "error", err)
		return ListProjectConversations200JSONResponse{}, nil
	}

	result := make(ListProjectConversations200JSONResponse, len(convs))
	for i, c := range convs {
		result[i] = domainConvToAPI(c)
	}
	return result, nil
}

// ListProjectArtifacts implements StrictServerInterface.
func (s *Server) ListProjectArtifacts(ctx context.Context, request ListProjectArtifactsRequestObject) (ListProjectArtifactsResponseObject, error) {
	arts, err := s.repo.ListProjectArtifacts(ctx, domain.ProjectID(request.Id))
	if err != nil {
		s.logger.Error("failed to list project artifacts", "error", err)
		return ListProjectArtifacts200JSONResponse{}, nil
	}

	result := make(ListProjectArtifacts200JSONResponse, len(arts))
	for i, a := range arts {
		result[i] = domainArtifactToAPI(a)
	}
	return result, nil
}

// --- StrictServerInterface implementations for Artifacts ---

// ListArtifacts implements StrictServerInterface.
func (s *Server) ListArtifacts(ctx context.Context, request ListArtifactsRequestObject) (ListArtifactsResponseObject, error) {
	arts, err := s.repo.ListArtifacts(ctx)
	if err != nil {
		s.logger.Error("failed to list artifacts", "error", err)
		return ListArtifacts200JSONResponse{}, nil
	}

	// Filter by type if query param provided
	var filtered []domain.Artifact
	if request.Params.Type != nil {
		filterType := domain.ArtifactType(string(*request.Params.Type))
		for _, a := range arts {
			if a.Type == filterType {
				filtered = append(filtered, a)
			}
		}
	} else {
		filtered = arts
	}

	result := make(ListArtifacts200JSONResponse, len(filtered))
	for i, a := range filtered {
		result[i] = domainArtifactToAPI(a)
	}
	return result, nil
}

// GetArtifact implements StrictServerInterface.
func (s *Server) GetArtifact(ctx context.Context, request GetArtifactRequestObject) (GetArtifactResponseObject, error) {
	art, err := s.repo.GetArtifact(ctx, domain.ArtifactID(request.Id))
	if err != nil {
		if err == domain.ErrArtifactNotFound {
			msg := "artifact not found"
			return GetArtifact404JSONResponse{Error: &msg}, nil
		}
		s.logger.Error("failed to get artifact", "error", err)
		return nil, err
	}
	return GetArtifact200JSONResponse(domainArtifactToAPI(art)), nil
}

// DeleteArtifact implements StrictServerInterface.
func (s *Server) DeleteArtifact(ctx context.Context, request DeleteArtifactRequestObject) (DeleteArtifactResponseObject, error) {
	if err := s.repo.DeleteArtifact(ctx, domain.ArtifactID(request.Id)); err != nil {
		if err == domain.ErrArtifactNotFound {
			return DeleteArtifact404Response{}, nil
		}
		s.logger.Error("failed to delete artifact", "error", err)
		return nil, err
	}
	return DeleteArtifact204Response{}, nil
}

// --- Mapping helpers ---

func domainProjectToAPI(p domain.Project) Project {
	id := string(p.ID)
	name := p.Name
	desc := p.Description
	return Project{
		Id:          &id,
		Name:        &name,
		Description: &desc,
		CreatedAt:   &p.CreatedAt,
		UpdatedAt:   &p.UpdatedAt,
	}
}

func domainArtifactToAPI(a domain.Artifact) Artifact {
	id := string(a.ID)
	artType := ArtifactType(a.Type)
	name := a.Name
	filePath := a.FilePath
	mimeType := a.MimeType
	prompt := a.Prompt

	art := Artifact{
		Id:        &id,
		Type:      &artType,
		Name:      &name,
		FilePath:  &filePath,
		MimeType:  &mimeType,
		SizeBytes: &a.SizeBytes,
		Prompt:    &prompt,
		CreatedAt: &a.CreatedAt,
	}

	if a.ProjectID != nil {
		s := string(*a.ProjectID)
		art.ProjectId = &s
	}
	if a.JobID != nil {
		s := string(*a.JobID)
		art.JobId = &s
	}
	if a.ConversationID != nil {
		s := string(*a.ConversationID)
		art.ConversationId = &s
	}

	return art
}
