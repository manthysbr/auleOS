package kernel

import (
	"context"
	"time"

	"github.com/manthysbr/auleOS/internal/core/domain"
)

// domainPersonaToAPI converts a domain Persona to an API Persona
func domainPersonaToAPI(p domain.Persona) Persona {
	id := string(p.ID)
	name := p.Name
	desc := p.Description
	prompt := p.SystemPrompt
	icon := p.Icon
	color := p.Color
	builtin := p.IsBuiltin
	createdAt := p.CreatedAt
	updatedAt := p.UpdatedAt

	var allowed *[]string
	if len(p.AllowedTools) > 0 {
		allowed = &p.AllowedTools
	}

	var modelOverride *string
	if p.ModelOverride != "" {
		mo := p.ModelOverride
		modelOverride = &mo
	}

	return Persona{
		Id:            &id,
		Name:          &name,
		Description:   &desc,
		SystemPrompt:  &prompt,
		Icon:          &icon,
		Color:         &color,
		AllowedTools:  allowed,
		ModelOverride: modelOverride,
		IsBuiltin:     &builtin,
		CreatedAt:     &createdAt,
		UpdatedAt:     &updatedAt,
	}
}

// ListPersonas implements StrictServerInterface
func (s *Server) ListPersonas(ctx context.Context, _ ListPersonasRequestObject) (ListPersonasResponseObject, error) {
	personas, err := s.repo.ListPersonas(ctx)
	if err != nil {
		s.logger.Error("failed to list personas", "error", err)
		return ListPersonas200JSONResponse([]Persona{}), nil
	}

	result := make([]Persona, 0, len(personas))
	for _, p := range personas {
		result = append(result, domainPersonaToAPI(p))
	}
	return ListPersonas200JSONResponse(result), nil
}

// CreatePersona implements StrictServerInterface
func (s *Server) CreatePersona(ctx context.Context, request CreatePersonaRequestObject) (CreatePersonaResponseObject, error) {
	now := time.Now()
	p := domain.Persona{
		ID:           domain.NewPersonaID(),
		Name:         request.Body.Name,
		SystemPrompt: request.Body.SystemPrompt,
		IsBuiltin:    false,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if request.Body.Description != nil {
		p.Description = *request.Body.Description
	}
	if request.Body.Icon != nil {
		p.Icon = *request.Body.Icon
	} else {
		p.Icon = "bot"
	}
	if request.Body.Color != nil {
		p.Color = *request.Body.Color
	} else {
		p.Color = "blue"
	}
	if request.Body.AllowedTools != nil {
		p.AllowedTools = *request.Body.AllowedTools
	}
	if request.Body.ModelOverride != nil {
		p.ModelOverride = *request.Body.ModelOverride
	}

	if err := s.repo.CreatePersona(ctx, p); err != nil {
		s.logger.Error("failed to create persona", "error", err)
		return CreatePersona201JSONResponse(domainPersonaToAPI(p)), nil
	}

	return CreatePersona201JSONResponse(domainPersonaToAPI(p)), nil
}

// GetPersona implements StrictServerInterface
func (s *Server) GetPersona(ctx context.Context, request GetPersonaRequestObject) (GetPersonaResponseObject, error) {
	p, err := s.repo.GetPersona(ctx, domain.PersonaID(request.Id))
	if err != nil {
		if err == domain.ErrPersonaNotFound {
			return GetPersona404JSONResponse{Error: strPtr("persona not found")}, nil
		}
		s.logger.Error("failed to get persona", "error", err)
		return GetPersona404JSONResponse{Error: strPtr(err.Error())}, nil
	}
	return GetPersona200JSONResponse(domainPersonaToAPI(p)), nil
}

// UpdatePersona implements StrictServerInterface
func (s *Server) UpdatePersona(ctx context.Context, request UpdatePersonaRequestObject) (UpdatePersonaResponseObject, error) {
	existing, err := s.repo.GetPersona(ctx, domain.PersonaID(request.Id))
	if err != nil {
		if err == domain.ErrPersonaNotFound {
			return UpdatePersona404JSONResponse{Error: strPtr("persona not found")}, nil
		}
		return UpdatePersona404JSONResponse{Error: strPtr(err.Error())}, nil
	}

	if request.Body.Name != nil {
		existing.Name = *request.Body.Name
	}
	if request.Body.Description != nil {
		existing.Description = *request.Body.Description
	}
	if request.Body.SystemPrompt != nil {
		existing.SystemPrompt = *request.Body.SystemPrompt
	}
	if request.Body.Icon != nil {
		existing.Icon = *request.Body.Icon
	}
	if request.Body.Color != nil {
		existing.Color = *request.Body.Color
	}
	if request.Body.AllowedTools != nil {
		existing.AllowedTools = *request.Body.AllowedTools
	}
	if request.Body.ModelOverride != nil {
		existing.ModelOverride = *request.Body.ModelOverride
	}
	existing.UpdatedAt = time.Now()

	if err := s.repo.UpdatePersona(ctx, existing); err != nil {
		s.logger.Error("failed to update persona", "error", err)
		return UpdatePersona404JSONResponse{Error: strPtr(err.Error())}, nil
	}

	return UpdatePersona200JSONResponse(domainPersonaToAPI(existing)), nil
}

// DeletePersona implements StrictServerInterface
func (s *Server) DeletePersona(ctx context.Context, request DeletePersonaRequestObject) (DeletePersonaResponseObject, error) {
	// Prevent deleting builtin personas
	p, err := s.repo.GetPersona(ctx, domain.PersonaID(request.Id))
	if err != nil {
		if err == domain.ErrPersonaNotFound {
			return DeletePersona404Response{}, nil
		}
		return DeletePersona404Response{}, nil
	}
	if p.IsBuiltin {
		return DeletePersona404Response{}, nil // refuse silently
	}

	if err := s.repo.DeletePersona(ctx, domain.PersonaID(request.Id)); err != nil {
		return DeletePersona404Response{}, nil
	}
	return DeletePersona204Response{}, nil
}

// strPtr is a helper to create a *string from a string literal
func strPtr(s string) *string {
	return &s
}
