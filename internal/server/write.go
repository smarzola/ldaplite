package server

import (
	"context"
	"log/slog"
	"time"

	"github.com/lor00x/goldap/message"

	"github.com/smarzola/ldaplite/internal/ldapdn"
	"github.com/smarzola/ldaplite/internal/models"
	"github.com/smarzola/ldaplite/internal/protocol"
)

// handleAdd handles add operations
func (s *Server) handleAdd(ctx context.Context, conn *protocol.Connection, msg *message.LDAPMessage) error {
	addReq := msg.ProtocolOp().(message.AddRequest)

	dn := string(addReq.Entry())
	slog.Debug("Add request", "dn", dn)

	if !s.canWrite(conn) {
		slog.Info("Add rejected - authenticated bind required", "dn", dn)
		return conn.WriteResponse(msg.MessageID(), protocol.NewAddResponse(message.ResultCodeInsufficientAccessRights))
	}

	// Check if entry already exists
	exists, err := s.store.EntryExists(ctx, dn)
	if err != nil {
		slog.Error("Failed to check entry existence", "dn", dn, "error", err)
		return conn.WriteResponse(msg.MessageID(), protocol.NewAddResponse(message.ResultCodeOperationsError))
	}

	if exists {
		slog.Debug("Entry already exists", "dn", dn)
		return conn.WriteResponse(msg.MessageID(), protocol.NewAddResponse(message.ResultCodeEntryAlreadyExists))
	}

	// Create new entry
	entry := &models.Entry{
		DN:         dn,
		ParentDN:   ldapdn.Parent(dn),
		Attributes: make(map[string][]string),
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	// Parse attributes
	attrs := addReq.Attributes()
	for _, attr := range attrs {
		name := string(attr.Type_())

		// Check protected attributes
		if isAddProtectedAttribute(name) {
			slog.Debug("Attempt to set protected attribute", "dn", dn, "attribute", name)
			return conn.WriteResponse(msg.MessageID(), protocol.NewAddResponse(message.ResultCodeUnwillingToPerform))
		}

		values := attr.Vals()
		for _, val := range values {
			entry.AddAttribute(name, string(val))
		}
	}

	// Process userPassword
	if userPassword := entry.GetAttribute("userPassword"); userPassword != "" {
		processedPassword, err := s.hasher.ProcessPassword(userPassword)
		if err != nil {
			slog.Debug("Invalid password format", "dn", dn, "error", err)
			return conn.WriteResponse(msg.MessageID(), protocol.NewAddResponse(message.ResultCodeConstraintViolation))
		}
		entry.SetAttribute("userPassword", processedPassword)
	}

	// Determine object class
	objectClasses := entry.GetAttribute("objectClass")
	if objectClasses == "" {
		slog.Debug("No objectClass provided", "dn", dn)
		return conn.WriteResponse(msg.MessageID(), protocol.NewAddResponse(message.ResultCodeObjectClassViolation))
	}

	entry.ObjectClass = objectClasses
	allClasses := entry.GetAttributes("objectClass")
	if len(allClasses) > 0 {
		entry.ObjectClass = allClasses[0]
	}

	slog.Debug("Creating entry", "dn", dn, "objectClass", objectClasses)

	// Store entry
	if err := s.store.CreateEntry(ctx, entry); err != nil {
		slog.Error("Failed to create entry", "dn", dn, "error", err)
		return conn.WriteResponse(msg.MessageID(), protocol.NewAddResponse(entryWriteResultCode(err)))
	}

	slog.Info("Entry created", "dn", dn)
	return conn.WriteResponse(msg.MessageID(), protocol.NewAddResponse(message.ResultCodeSuccess))
}

// handleDelete handles delete operations
func (s *Server) handleDelete(ctx context.Context, conn *protocol.Connection, msg *message.LDAPMessage) error {
	delReq := msg.ProtocolOp().(message.DelRequest)

	dn := string(delReq)
	slog.Debug("Delete request", "dn", dn)

	if !s.canWrite(conn) {
		slog.Info("Delete rejected - authenticated bind required", "dn", dn)
		return conn.WriteResponse(msg.MessageID(), protocol.NewDelResponse(message.ResultCodeInsufficientAccessRights))
	}

	// Check if entry exists
	exists, err := s.store.EntryExists(ctx, dn)
	if err != nil {
		slog.Error("Failed to check entry existence", "dn", dn, "error", err)
		return conn.WriteResponse(msg.MessageID(), protocol.NewDelResponse(message.ResultCodeOperationsError))
	}

	if !exists {
		slog.Debug("Entry not found", "dn", dn)
		return conn.WriteResponse(msg.MessageID(), protocol.NewDelResponse(message.ResultCodeNoSuchObject))
	}

	if err := s.store.DeleteEntry(ctx, dn); err != nil {
		slog.Error("Failed to delete entry", "dn", dn, "error", err)
		return conn.WriteResponse(msg.MessageID(), protocol.NewDelResponse(message.ResultCodeOperationsError))
	}

	slog.Info("Entry deleted", "dn", dn)
	return conn.WriteResponse(msg.MessageID(), protocol.NewDelResponse(message.ResultCodeSuccess))
}

// handleModify handles modify operations
func (s *Server) handleModify(ctx context.Context, conn *protocol.Connection, msg *message.LDAPMessage) error {
	modReq := msg.ProtocolOp().(message.ModifyRequest)

	dn := string(modReq.Object())
	slog.Debug("Modify request", "dn", dn)

	if !s.canWrite(conn) {
		slog.Info("Modify rejected - authenticated bind required", "dn", dn)
		return conn.WriteResponse(msg.MessageID(), protocol.NewModifyResponse(message.ResultCodeInsufficientAccessRights))
	}

	// Get entry
	entry, err := s.store.GetEntry(ctx, dn)
	if err != nil {
		slog.Error("Failed to get entry", "dn", dn, "error", err)
		return conn.WriteResponse(msg.MessageID(), protocol.NewModifyResponse(message.ResultCodeOperationsError))
	}

	if entry == nil {
		slog.Debug("Entry not found", "dn", dn)
		return conn.WriteResponse(msg.MessageID(), protocol.NewModifyResponse(message.ResultCodeNoSuchObject))
	}

	// Apply modifications
	changes := modReq.Changes()
	for _, change := range changes {
		modification := change.Modification()
		attrType := string(modification.Type_())

		// Check protected attributes
		if isModifyProtectedAttribute(attrType) {
			slog.Debug("Attempt to modify protected attribute", "dn", dn, "attribute", attrType)
			return conn.WriteResponse(msg.MessageID(), protocol.NewModifyResponse(message.ResultCodeUnwillingToPerform))
		}

		vals := attributeValues(modification.Vals())
		opType := int(change.Operation())

		switch opType {
		case 0: // Add
			slog.Debug("Add attribute", "attr", attrType)
			if err := s.addModifyValues(entry, attrType, vals); err != nil {
				slog.Debug("Invalid password format", "dn", dn, "error", err)
				return conn.WriteResponse(msg.MessageID(), protocol.NewModifyResponse(message.ResultCodeConstraintViolation))
			}

		case 1: // Delete
			slog.Debug("Delete attribute", "attr", attrType)
			deleteModifyValues(entry, attrType, vals)

		case 2: // Replace
			slog.Debug("Replace attribute", "attr", attrType)
			if err := s.replaceModifyValues(entry, attrType, vals); err != nil {
				slog.Debug("Invalid password format", "dn", dn, "error", err)
				return conn.WriteResponse(msg.MessageID(), protocol.NewModifyResponse(message.ResultCodeConstraintViolation))
			}
		}
	}

	// Update entry
	if err := s.store.UpdateEntry(ctx, entry); err != nil {
		slog.Error("Failed to update entry", "dn", dn, "error", err)
		return conn.WriteResponse(msg.MessageID(), protocol.NewModifyResponse(entryWriteResultCode(err)))
	}

	slog.Info("Entry modified", "dn", dn)
	return conn.WriteResponse(msg.MessageID(), protocol.NewModifyResponse(message.ResultCodeSuccess))
}

func attributeValues(vals []message.AttributeValue) []string {
	out := make([]string, 0, len(vals))
	for _, val := range vals {
		out = append(out, string(val))
	}
	return out
}

func (s *Server) addModifyValues(entry *models.Entry, attrType string, vals []string) error {
	if attrType == "userPassword" {
		for _, val := range vals {
			processedPassword, err := s.hasher.ProcessPassword(val)
			if err != nil {
				return err
			}
			entry.AddAttribute(attrType, processedPassword)
		}
		return nil
	}

	for _, val := range vals {
		entry.AddAttribute(attrType, val)
	}
	return nil
}

func deleteModifyValues(entry *models.Entry, attrType string, vals []string) {
	if len(vals) == 0 {
		entry.RemoveAttribute(attrType)
		return
	}

	remove := make(map[string]struct{}, len(vals))
	for _, val := range vals {
		remove[val] = struct{}{}
	}

	existing := entry.GetAttributes(attrType)
	entry.RemoveAttribute(attrType)
	for _, value := range existing {
		if _, ok := remove[value]; !ok {
			entry.AddAttribute(attrType, value)
		}
	}
}

func (s *Server) replaceModifyValues(entry *models.Entry, attrType string, vals []string) error {
	entry.RemoveAttribute(attrType)
	return s.addModifyValues(entry, attrType, vals)
}
