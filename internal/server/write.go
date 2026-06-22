package server

import (
	"context"
	"log/slog"
	"time"

	"github.com/smarzola/ldaplite/internal/audit"
	"github.com/smarzola/ldaplite/internal/ldapdn"
	"github.com/smarzola/ldaplite/internal/models"
	"github.com/smarzola/ldaplite/internal/protocol"
	"github.com/smarzola/ldaplite/internal/protocol/ldapmsg"
	"github.com/smarzola/ldaplite/internal/store"
)

// handleAdd handles add operations
func (s *Server) handleAdd(ctx context.Context, conn *protocol.Connection, msg *ldapmsg.Message) error {
	start := time.Now()
	addReq := msg.Op.(ldapmsg.AddRequest)

	dn := addReq.Entry
	resultCode := ldapmsg.ResultCodeOperationsError
	defer func() {
		s.auditLDAPOperation(ctx, conn, msg, "add", audit.LDAPEvent{
			ActorDN:    conn.GetBoundDN(),
			TargetDN:   dn,
			ResultCode: int(resultCode),
			Duration:   time.Since(start),
		})
	}()

	slog.Debug("Add request", "dn", dn)

	if !s.canWrite(conn) {
		slog.Info("Add rejected - authenticated bind required", "dn", dn)
		resultCode = ldapmsg.ResultCodeInsufficientAccessRights
		return conn.WriteResponse(msg.ID, protocol.NewAddResponse(ldapmsg.ResultCodeInsufficientAccessRights))
	}

	// Check if entry already exists
	exists, err := s.store.EntryExists(ctx, dn)
	if err != nil {
		slog.Error("Failed to check entry existence", "dn", dn, "error", err)
		resultCode = ldapmsg.ResultCodeOperationsError
		return conn.WriteResponse(msg.ID, protocol.NewAddResponse(ldapmsg.ResultCodeOperationsError))
	}

	if exists {
		slog.Debug("Entry already exists", "dn", dn)
		resultCode = ldapmsg.ResultCodeEntryAlreadyExists
		return conn.WriteResponse(msg.ID, protocol.NewAddResponse(ldapmsg.ResultCodeEntryAlreadyExists))
	}

	entry, resultCode, err := s.newAddEntry(dn, addRequestAttributes(addReq.Attributes))
	if err != nil {
		slog.Debug("Invalid add request", "dn", dn, "error", err)
		return conn.WriteResponse(msg.ID, protocol.NewAddResponse(resultCode))
	}
	if resultCode != ldapmsg.ResultCodeSuccess {
		if resultCode == ldapmsg.ResultCodeUnwillingToPerform {
			slog.Debug("Attempt to set protected attribute", "dn", dn)
		}
		return conn.WriteResponse(msg.ID, protocol.NewAddResponse(resultCode))
	}

	slog.Debug("Creating entry", "dn", dn, "objectClass", entry.ObjectClass)

	// Store entry
	if err := s.store.CreateEntry(ctx, entry); err != nil {
		slog.Error("Failed to create entry", "dn", dn, "error", err)
		resultCode = entryWriteResultCode(err)
		return conn.WriteResponse(msg.ID, protocol.NewAddResponse(entryWriteResultCode(err)))
	}

	slog.Info("Entry created", "dn", dn)
	resultCode = ldapmsg.ResultCodeSuccess
	return conn.WriteResponse(msg.ID, protocol.NewAddResponse(ldapmsg.ResultCodeSuccess))
}

// handleDelete handles delete operations
func (s *Server) handleDelete(ctx context.Context, conn *protocol.Connection, msg *ldapmsg.Message) error {
	start := time.Now()
	delReq := msg.Op.(ldapmsg.DeleteRequest)
	dn := delReq.DN
	resultCode := ldapmsg.ResultCodeOperationsError
	defer func() {
		s.auditLDAPOperation(ctx, conn, msg, "delete", audit.LDAPEvent{
			ActorDN:    conn.GetBoundDN(),
			TargetDN:   dn,
			ResultCode: int(resultCode),
			Duration:   time.Since(start),
		})
	}()

	slog.Debug("Delete request", "dn", dn)

	if !s.canWrite(conn) {
		slog.Info("Delete rejected - authenticated bind required", "dn", dn)
		resultCode = ldapmsg.ResultCodeInsufficientAccessRights
		return conn.WriteResponse(msg.ID, protocol.NewDelResponse(ldapmsg.ResultCodeInsufficientAccessRights))
	}

	// Check if entry exists
	exists, err := s.store.EntryExists(ctx, dn)
	if err != nil {
		slog.Error("Failed to check entry existence", "dn", dn, "error", err)
		resultCode = ldapmsg.ResultCodeOperationsError
		return conn.WriteResponse(msg.ID, protocol.NewDelResponse(ldapmsg.ResultCodeOperationsError))
	}

	if !exists {
		slog.Debug("Entry not found", "dn", dn)
		resultCode = ldapmsg.ResultCodeNoSuchObject
		return conn.WriteResponse(msg.ID, protocol.NewDelResponse(ldapmsg.ResultCodeNoSuchObject))
	}

	if err := s.store.DeleteEntry(ctx, dn); err != nil {
		slog.Error("Failed to delete entry", "dn", dn, "error", err)
		resultCode = ldapmsg.ResultCodeOperationsError
		return conn.WriteResponse(msg.ID, protocol.NewDelResponse(ldapmsg.ResultCodeOperationsError))
	}

	slog.Info("Entry deleted", "dn", dn)
	resultCode = ldapmsg.ResultCodeSuccess
	return conn.WriteResponse(msg.ID, protocol.NewDelResponse(ldapmsg.ResultCodeSuccess))
}

// handleModify handles modify operations
func (s *Server) handleModify(ctx context.Context, conn *protocol.Connection, msg *ldapmsg.Message) error {
	start := time.Now()
	modReq := msg.Op.(ldapmsg.ModifyRequest)

	dn := modReq.Object
	resultCode := ldapmsg.ResultCodeOperationsError
	defer func() {
		s.auditLDAPOperation(ctx, conn, msg, "modify", audit.LDAPEvent{
			ActorDN:    conn.GetBoundDN(),
			TargetDN:   dn,
			ResultCode: int(resultCode),
			Duration:   time.Since(start),
		})
	}()

	slog.Debug("Modify request", "dn", dn)

	if !s.canWrite(conn) {
		slog.Info("Modify rejected - authenticated bind required", "dn", dn)
		resultCode = ldapmsg.ResultCodeInsufficientAccessRights
		return conn.WriteResponse(msg.ID, protocol.NewModifyResponse(ldapmsg.ResultCodeInsufficientAccessRights))
	}

	// Get entry
	entry, err := s.store.GetEntryWithOptions(ctx, dn, store.EntryOptions{IncludeMemberOf: false})
	if err != nil {
		slog.Error("Failed to get entry", "dn", dn, "error", err)
		resultCode = ldapmsg.ResultCodeOperationsError
		return conn.WriteResponse(msg.ID, protocol.NewModifyResponse(ldapmsg.ResultCodeOperationsError))
	}

	if entry == nil {
		slog.Debug("Entry not found", "dn", dn)
		resultCode = ldapmsg.ResultCodeNoSuchObject
		return conn.WriteResponse(msg.ID, protocol.NewModifyResponse(ldapmsg.ResultCodeNoSuchObject))
	}

	// Apply modifications
	changes := modReq.Changes
	for _, change := range changes {
		modification := change.Modification
		attrType := modification.Name

		// Check protected attributes
		if isModifyProtectedAttribute(attrType) {
			slog.Debug("Attempt to modify protected attribute", "dn", dn, "attribute", attrType)
			resultCode = ldapmsg.ResultCodeUnwillingToPerform
			return conn.WriteResponse(msg.ID, protocol.NewModifyResponse(ldapmsg.ResultCodeUnwillingToPerform))
		}

		vals := modification.Values

		switch change.Operation {
		case ldapmsg.ModifyOperationAdd:
			slog.Debug("Add attribute", "attr", attrType)
			if err := s.addModifyValues(entry, attrType, vals); err != nil {
				slog.Debug("Invalid password format", "dn", dn, "error", err)
				resultCode = ldapmsg.ResultCodeConstraintViolation
				return conn.WriteResponse(msg.ID, protocol.NewModifyResponse(ldapmsg.ResultCodeConstraintViolation))
			}

		case ldapmsg.ModifyOperationDelete:
			slog.Debug("Delete attribute", "attr", attrType)
			deleteModifyValues(entry, attrType, vals)

		case ldapmsg.ModifyOperationReplace:
			slog.Debug("Replace attribute", "attr", attrType)
			if err := s.replaceModifyValues(entry, attrType, vals); err != nil {
				slog.Debug("Invalid password format", "dn", dn, "error", err)
				resultCode = ldapmsg.ResultCodeConstraintViolation
				return conn.WriteResponse(msg.ID, protocol.NewModifyResponse(ldapmsg.ResultCodeConstraintViolation))
			}
		}
	}

	// Update entry
	if err := s.store.UpdateEntry(ctx, entry); err != nil {
		slog.Error("Failed to update entry", "dn", dn, "error", err)
		resultCode = entryWriteResultCode(err)
		return conn.WriteResponse(msg.ID, protocol.NewModifyResponse(entryWriteResultCode(err)))
	}

	slog.Info("Entry modified", "dn", dn)
	resultCode = ldapmsg.ResultCodeSuccess
	return conn.WriteResponse(msg.ID, protocol.NewModifyResponse(ldapmsg.ResultCodeSuccess))
}

func addRequestAttributes(attrs []ldapmsg.Attribute) map[string][]string {
	values := make(map[string][]string, len(attrs))
	for _, attr := range attrs {
		values[attr.Name] = append(values[attr.Name], attr.Values...)
	}
	return values
}

func (s *Server) newAddEntry(dn string, attrs map[string][]string) (*models.Entry, ldapmsg.ResultCode, error) {
	entry := &models.Entry{
		DN:         dn,
		ParentDN:   ldapdn.Parent(dn),
		Attributes: make(map[string][]string),
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	for name, values := range attrs {
		if isAddProtectedAttribute(name) {
			return nil, ldapmsg.ResultCodeUnwillingToPerform, nil
		}
		for _, value := range values {
			entry.AddAttribute(name, value)
		}
	}

	if userPassword := entry.GetAttribute("userPassword"); userPassword != "" {
		processedPassword, err := s.hasher.ProcessPassword(userPassword)
		if err != nil {
			return nil, ldapmsg.ResultCodeConstraintViolation, err
		}
		entry.SetAttribute("userPassword", processedPassword)
	}

	objectClass := entry.GetAttribute("objectClass")
	if objectClass == "" {
		return nil, ldapmsg.ResultCodeObjectClassViolation, nil
	}
	allClasses := entry.GetAttributes("objectClass")
	if len(allClasses) > 0 {
		objectClass = allClasses[0]
	}
	entry.ObjectClass = objectClass
	delete(entry.Attributes, "objectclass")

	return entry, ldapmsg.ResultCodeSuccess, nil
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
