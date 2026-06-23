package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/smarzola/ldaplite/internal/directory"
	"github.com/smarzola/ldaplite/internal/store"
	"github.com/smarzola/ldaplite/internal/web/middleware"
)

type passwordRequest struct {
	DN       string `json:"dn"`
	Password string `json:"password"`
}

func (h *APIHandler) Users(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		var input directory.UserInput
		if !decodeJSON(w, r, &input) {
			return
		}
		entry, err := h.service.CreateUser(r.Context(), input)
		if err != nil {
			writeAPIError(w, err)
			auditWebWrite(r, "create", "user", "", statusForError(err), err)
			return
		}
		auditWebWrite(r, "create", "user", entry.DN, http.StatusCreated, nil)
		writeJSONStatus(w, http.StatusCreated, summarizeEntry(entry))
	case http.MethodPut:
		dn := r.URL.Query().Get("dn")
		var input directory.UserInput
		if !decodeJSON(w, r, &input) {
			return
		}
		entry, err := h.service.UpdateUser(r.Context(), dn, input)
		if err != nil {
			writeAPIError(w, err)
			auditWebWrite(r, "update", "user", dn, statusForError(err), err)
			return
		}
		auditWebWrite(r, "update", "user", entry.DN, http.StatusOK, nil)
		writeJSON(w, summarizeEntry(entry))
	case http.MethodDelete:
		dn := r.URL.Query().Get("dn")
		if err := h.service.DeleteEntry(r.Context(), dn); err != nil {
			writeAPIError(w, err)
			auditWebWrite(r, "delete", "user", dn, statusForError(err), err)
			return
		}
		auditWebWrite(r, "delete", "user", dn, http.StatusNoContent, nil)
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *APIHandler) Groups(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		var input directory.GroupInput
		if !decodeJSON(w, r, &input) {
			return
		}
		entry, err := h.service.CreateGroup(r.Context(), input)
		if err != nil {
			writeAPIError(w, err)
			auditWebWrite(r, "create", "group", "", statusForError(err), err)
			return
		}
		auditWebWrite(r, "create", "group", entry.DN, http.StatusCreated, nil)
		writeJSONStatus(w, http.StatusCreated, summarizeEntry(entry))
	case http.MethodPut:
		dn := r.URL.Query().Get("dn")
		var input directory.GroupInput
		if !decodeJSON(w, r, &input) {
			return
		}
		entry, err := h.service.UpdateGroup(r.Context(), dn, input)
		if err != nil {
			writeAPIError(w, err)
			auditWebWrite(r, "update", "group", dn, statusForError(err), err)
			return
		}
		auditWebWrite(r, "update", "group", entry.DN, http.StatusOK, nil)
		writeJSON(w, summarizeEntry(entry))
	case http.MethodDelete:
		dn := r.URL.Query().Get("dn")
		if err := h.service.DeleteEntry(r.Context(), dn); err != nil {
			writeAPIError(w, err)
			auditWebWrite(r, "delete", "group", dn, statusForError(err), err)
			return
		}
		auditWebWrite(r, "delete", "group", dn, http.StatusNoContent, nil)
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *APIHandler) OUs(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		var input directory.OUInput
		if !decodeJSON(w, r, &input) {
			return
		}
		entry, err := h.service.CreateOU(r.Context(), input)
		if err != nil {
			writeAPIError(w, err)
			auditWebWrite(r, "create", "ou", "", statusForError(err), err)
			return
		}
		auditWebWrite(r, "create", "ou", entry.DN, http.StatusCreated, nil)
		writeJSONStatus(w, http.StatusCreated, summarizeEntry(entry))
	case http.MethodPut:
		dn := r.URL.Query().Get("dn")
		var input directory.OUInput
		if !decodeJSON(w, r, &input) {
			return
		}
		entry, err := h.service.UpdateOU(r.Context(), dn, input)
		if err != nil {
			writeAPIError(w, err)
			auditWebWrite(r, "update", "ou", dn, statusForError(err), err)
			return
		}
		auditWebWrite(r, "update", "ou", entry.DN, http.StatusOK, nil)
		writeJSON(w, summarizeEntry(entry))
	case http.MethodDelete:
		dn := r.URL.Query().Get("dn")
		if err := h.service.DeleteEntry(r.Context(), dn); err != nil {
			writeAPIError(w, err)
			auditWebWrite(r, "delete", "ou", dn, statusForError(err), err)
			return
		}
		auditWebWrite(r, "delete", "ou", dn, http.StatusNoContent, nil)
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *APIHandler) ChangeOwnPassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var input passwordRequest
	if !decodeJSON(w, r, &input) {
		return
	}

	userDN := middleware.GetUserDN(r)
	if err := h.service.ChangeOwnPassword(r.Context(), userDN, input.Password); err != nil {
		writeAPIError(w, err)
		auditWebWrite(r, "change-password", "account", userDN, statusForError(err), err)
		return
	}
	auditWebWrite(r, "change-password", "account", userDN, http.StatusNoContent, nil)
	w.WriteHeader(http.StatusNoContent)
}

func (h *APIHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var input passwordRequest
	if !decodeJSON(w, r, &input) {
		return
	}

	if err := h.service.ResetPassword(r.Context(), input.DN, input.Password); err != nil {
		writeAPIError(w, err)
		auditWebWrite(r, "reset-password", "user", input.DN, statusForError(err), err)
		return
	}
	auditWebWrite(r, "reset-password", "user", input.DN, http.StatusNoContent, nil)
	w.WriteHeader(http.StatusNoContent)
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	defer r.Body.Close()
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		http.Error(w, "Invalid JSON request", http.StatusBadRequest)
		return false
	}
	return true
}

func writeJSONStatus(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeAPIError(w http.ResponseWriter, err error) {
	http.Error(w, err.Error(), statusForError(err))
}

func statusForError(err error) int {
	switch {
	case errors.Is(err, directory.ErrInvalidRequest),
		errors.Is(err, directory.ErrProtectedAttribute),
		errors.Is(err, directory.ErrUnsupportedObject),
		errors.Is(err, directory.ErrPasswordNotProvided),
		errors.Is(err, store.ErrConstraintViolation),
		errors.Is(err, store.ErrObjectClassViolation):
		return http.StatusBadRequest
	case errors.Is(err, store.ErrNoSuchObject):
		return http.StatusNotFound
	case errors.Is(err, store.ErrEntryAlreadyExists):
		return http.StatusConflict
	default:
		return http.StatusInternalServerError
	}
}
