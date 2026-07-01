package ldif

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/smarzola/ldaplite/internal/ldapdn"
	"github.com/smarzola/ldaplite/internal/models"
	"github.com/smarzola/ldaplite/pkg/crypto"
)

// EntryLookup is the store capability needed for import validation.
type EntryLookup interface {
	EntryExists(ctx context.Context, dn string) (bool, error)
}

// ImportPlanOptions configures LDIF import planning.
type ImportPlanOptions struct {
	BaseDN string
	Hasher *crypto.PasswordHasher
}

// ImportPlan is a validated, ordered set of entries ready for a later write step.
type ImportPlan struct {
	Entries []*models.Entry
}

// ImportPlanError reports a validation error for one LDIF record.
type ImportPlanError struct {
	DN  string
	Msg string
}

func (e *ImportPlanError) Error() string {
	if e.DN != "" {
		return fmt.Sprintf("%s: %s", e.DN, e.Msg)
	}
	return e.Msg
}

// PlanImport validates LDIF records without mutating storage.
func PlanImport(ctx context.Context, lookup EntryLookup, records []Record, options ImportPlanOptions) (*ImportPlan, error) {
	baseDN := strings.TrimSpace(options.BaseDN)
	if baseDN == "" {
		return nil, &ImportPlanError{Msg: "base DN is required"}
	}
	if lookup == nil {
		return nil, &ImportPlanError{Msg: "entry lookup is required"}
	}
	if options.Hasher == nil {
		return nil, &ImportPlanError{Msg: "password hasher is required"}
	}

	batchDNs := make(map[string]struct{}, len(records))
	for _, record := range records {
		key := dnKey(record.DN)
		if key == "" {
			return nil, &ImportPlanError{Msg: "record is missing dn"}
		}
		if _, exists := batchDNs[key]; exists {
			return nil, &ImportPlanError{DN: record.DN, Msg: "duplicate DN in import batch"}
		}
		batchDNs[key] = struct{}{}
	}

	entries := make([]*models.Entry, 0, len(records))
	for _, record := range records {
		entry, err := entryFromRecord(record, baseDN, options.Hasher)
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}

	for _, entry := range entries {
		if err := validateParent(ctx, lookup, batchDNs, baseDN, entry); err != nil {
			return nil, err
		}
		if err := validateGroupMembers(ctx, lookup, batchDNs, entry); err != nil {
			return nil, err
		}
	}

	sort.SliceStable(entries, func(i, j int) bool {
		return dnDepth(entries[i].DN) < dnDepth(entries[j].DN)
	})

	return &ImportPlan{Entries: entries}, nil
}

func entryFromRecord(record Record, baseDN string, hasher *crypto.PasswordHasher) (*models.Entry, error) {
	if !ldapdn.WithinBase(record.DN, baseDN) {
		return nil, &ImportPlanError{DN: record.DN, Msg: fmt.Sprintf("DN is outside base DN %s", baseDN)}
	}

	objectClass, err := primaryObjectClass(record, baseDN)
	if err != nil {
		return nil, err
	}
	if err := rejectProtectedAttributes(record); err != nil {
		return nil, err
	}

	entry := models.NewEntry(record.DN, objectClass)
	for _, attr := range record.Attributes {
		if strings.EqualFold(attr.Name, "objectClass") {
			continue
		}
		if strings.EqualFold(attr.Name, "userPassword") {
			continue
		}
		entry.AddAttribute(attr.Name, attr.Value)
	}

	if entry.IsUser() {
		passwords := record.Values("userPassword")
		if len(passwords) == 0 {
			return nil, &ImportPlanError{DN: record.DN, Msg: "userPassword is required for user import"}
		}
		if len(passwords) > 1 {
			return nil, &ImportPlanError{DN: record.DN, Msg: "userPassword must be single-valued"}
		}
		processed, err := hasher.ProcessPassword(passwords[0])
		if err != nil {
			return nil, &ImportPlanError{DN: record.DN, Msg: err.Error()}
		}
		entry.SetAttribute("userPassword", processed)
	} else if len(record.Values("userPassword")) > 0 {
		return nil, &ImportPlanError{DN: record.DN, Msg: "userPassword is only supported on inetOrgPerson entries"}
	}

	if err := validateModel(entry); err != nil {
		return nil, &ImportPlanError{DN: record.DN, Msg: err.Error()}
	}
	return entry, nil
}

func primaryObjectClass(record Record, baseDN string) (string, error) {
	values := record.Values("objectClass")
	if len(values) == 0 {
		return "", &ImportPlanError{DN: record.DN, Msg: "objectClass is required"}
	}

	var primary string
	for _, value := range values {
		switch strings.ToLower(strings.TrimSpace(value)) {
		case strings.ToLower(string(models.ObjectClassTop)):
			if ldapdn.Equal(record.DN, baseDN) && primary == "" {
				primary = string(models.ObjectClassTop)
			}
		case strings.ToLower(string(models.ObjectClassOrganizationalUnit)):
			if primary != "" && primary != string(models.ObjectClassTop) {
				return "", &ImportPlanError{DN: record.DN, Msg: "multiple supported structural objectClass values"}
			}
			primary = string(models.ObjectClassOrganizationalUnit)
		case strings.ToLower(string(models.ObjectClassInetOrgPerson)):
			if primary != "" && primary != string(models.ObjectClassTop) {
				return "", &ImportPlanError{DN: record.DN, Msg: "multiple supported structural objectClass values"}
			}
			primary = string(models.ObjectClassInetOrgPerson)
		case strings.ToLower(string(models.ObjectClassGroupOfNames)):
			if primary != "" && primary != string(models.ObjectClassTop) {
				return "", &ImportPlanError{DN: record.DN, Msg: "multiple supported structural objectClass values"}
			}
			primary = string(models.ObjectClassGroupOfNames)
		default:
			return "", &ImportPlanError{DN: record.DN, Msg: fmt.Sprintf("unsupported objectClass %q", value)}
		}
	}

	if primary == "" || (primary == string(models.ObjectClassTop) && !ldapdn.Equal(record.DN, baseDN)) {
		return "", &ImportPlanError{DN: record.DN, Msg: "exactly one supported structural objectClass is required"}
	}
	return primary, nil
}

func rejectProtectedAttributes(record Record) error {
	for _, attr := range record.Attributes {
		switch strings.ToLower(attr.Name) {
		case "entryuuid", "uuid", "createtimestamp", "modifytimestamp", "memberof":
			return &ImportPlanError{DN: record.DN, Msg: fmt.Sprintf("protected attribute %s is not importable", attr.Name)}
		}
	}
	return nil
}

func validateModel(entry *models.Entry) error {
	switch {
	case entry.IsUser():
		return (&models.User{Entry: entry, UID: entry.GetAttribute("uid")}).ValidateUser()
	case entry.IsGroup():
		return (&models.Group{Entry: entry, CN: entry.GetAttribute("cn")}).ValidateGroup()
	case entry.IsOrganizationalUnit():
		return (&models.OrganizationalUnit{Entry: entry, OU: entry.GetAttribute("ou")}).ValidateOU()
	default:
		return entry.Validate()
	}
}

func validateParent(ctx context.Context, lookup EntryLookup, batchDNs map[string]struct{}, baseDN string, entry *models.Entry) error {
	if ldapdn.Equal(entry.DN, baseDN) {
		return nil
	}
	parentDN := strings.TrimSpace(entry.ParentDN)
	if parentDN == "" {
		return &ImportPlanError{DN: entry.DN, Msg: "parent DN is required"}
	}
	if _, ok := batchDNs[dnKey(parentDN)]; ok {
		return nil
	}
	exists, err := lookup.EntryExists(ctx, parentDN)
	if err != nil {
		return &ImportPlanError{DN: entry.DN, Msg: fmt.Sprintf("failed to check parent DN %s: %v", parentDN, err)}
	}
	if !exists {
		return &ImportPlanError{DN: entry.DN, Msg: fmt.Sprintf("parent DN does not exist: %s", parentDN)}
	}
	return nil
}

func validateGroupMembers(ctx context.Context, lookup EntryLookup, batchDNs map[string]struct{}, entry *models.Entry) error {
	if !entry.IsGroup() {
		return nil
	}
	for _, memberDN := range entry.GetAttributes("member") {
		if _, ok := batchDNs[dnKey(memberDN)]; ok {
			continue
		}
		exists, err := lookup.EntryExists(ctx, memberDN)
		if err != nil {
			return &ImportPlanError{DN: entry.DN, Msg: fmt.Sprintf("failed to check group member %s: %v", memberDN, err)}
		}
		if !exists {
			return &ImportPlanError{DN: entry.DN, Msg: fmt.Sprintf("group member does not exist: %s", memberDN)}
		}
	}
	return nil
}

func dnKey(dn string) string {
	return strings.ToLower(strings.TrimSpace(dn))
}

func dnDepth(dn string) int {
	if strings.TrimSpace(dn) == "" {
		return 0
	}
	return strings.Count(dn, ",") + 1
}
