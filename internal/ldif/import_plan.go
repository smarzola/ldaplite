package ldif

import (
	"context"
	crand "crypto/rand"
	"encoding/base64"
	"fmt"
	"sort"
	"strings"

	"github.com/smarzola/ldaplite/internal/ldapdn"
	"github.com/smarzola/ldaplite/internal/models"
	"github.com/smarzola/ldaplite/internal/store"
	"github.com/smarzola/ldaplite/pkg/crypto"
)

// EntryLookup is the store capability needed for import validation.
type EntryLookup interface {
	EntryExists(ctx context.Context, dn string) (bool, error)
}

// ImportPlanOptions configures LDIF import planning.
type ImportPlanOptions struct {
	BaseDN                  string
	Hasher                  *crypto.PasswordHasher
	ReplaceExisting         bool
	AllowGeneratedPasswords bool
}

// ImportPlan is a validated, ordered set of entries ready for a later write step.
type ImportPlan struct {
	Entries            []*models.Entry
	ReplaceExisting    bool
	GeneratedPasswords []GeneratedPassword
}

// GeneratedPassword is a plaintext password generated for a missing userPassword.
type GeneratedPassword struct {
	DN       string
	Password string
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
	generatedPasswords := make([]GeneratedPassword, 0)
	for _, record := range records {
		entry, generated, err := entryFromRecord(record, baseDN, options)
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
		if generated != nil {
			generatedPasswords = append(generatedPasswords, *generated)
		}
	}

	for _, entry := range entries {
		if err := validateExistingEntry(ctx, lookup, options, entry); err != nil {
			return nil, err
		}
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

	return &ImportPlan{
		Entries:            entries,
		ReplaceExisting:    options.ReplaceExisting,
		GeneratedPasswords: generatedPasswords,
	}, nil
}

func entryFromRecord(record Record, baseDN string, options ImportPlanOptions) (*models.Entry, *GeneratedPassword, error) {
	if !ldapdn.WithinBase(record.DN, baseDN) {
		return nil, nil, &ImportPlanError{DN: record.DN, Msg: fmt.Sprintf("DN is outside base DN %s", baseDN)}
	}

	objectClass, err := primaryObjectClass(record, baseDN)
	if err != nil {
		return nil, nil, err
	}
	if err := rejectProtectedAttributes(record); err != nil {
		return nil, nil, err
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

	var generated *GeneratedPassword
	if entry.IsUser() {
		passwords := record.Values("userPassword")
		if len(passwords) == 0 {
			if !options.AllowGeneratedPasswords {
				return nil, nil, &ImportPlanError{DN: record.DN, Msg: "userPassword is required for user import"}
			}
			password, err := generatePassword()
			if err != nil {
				return nil, nil, &ImportPlanError{DN: record.DN, Msg: fmt.Sprintf("failed to generate password: %v", err)}
			}
			passwords = []string{password}
			generated = &GeneratedPassword{DN: record.DN, Password: password}
		}
		if len(passwords) > 1 {
			return nil, nil, &ImportPlanError{DN: record.DN, Msg: "userPassword must be single-valued"}
		}
		processed, err := options.Hasher.ProcessPassword(passwords[0])
		if err != nil {
			return nil, nil, &ImportPlanError{DN: record.DN, Msg: err.Error()}
		}
		entry.SetAttribute("userPassword", processed)
	} else if len(record.Values("userPassword")) > 0 {
		return nil, nil, &ImportPlanError{DN: record.DN, Msg: "userPassword is only supported on inetOrgPerson entries"}
	}

	if err := validateModel(entry); err != nil {
		return nil, nil, &ImportPlanError{DN: record.DN, Msg: err.Error()}
	}
	return entry, generated, nil
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

func validateExistingEntry(ctx context.Context, lookup EntryLookup, options ImportPlanOptions, entry *models.Entry) error {
	exists, err := lookup.EntryExists(ctx, entry.DN)
	if err != nil {
		return &ImportPlanError{DN: entry.DN, Msg: fmt.Sprintf("failed to check existing entry: %v", err)}
	}
	if !exists {
		return nil
	}
	if !options.ReplaceExisting {
		return &ImportPlanError{DN: entry.DN, Msg: "entry already exists"}
	}
	reader, ok := lookup.(interface {
		GetEntryWithOptions(ctx context.Context, dn string, options store.EntryOptions) (*models.Entry, error)
	})
	if !ok {
		return nil
	}
	current, err := reader.GetEntryWithOptions(ctx, entry.DN, store.EntryOptions{IncludeMemberOf: false})
	if err != nil {
		return &ImportPlanError{DN: entry.DN, Msg: fmt.Sprintf("failed to read existing entry: %v", err)}
	}
	if current != nil && current.ObjectClass != entry.ObjectClass {
		return &ImportPlanError{DN: entry.DN, Msg: fmt.Sprintf("cannot replace %s entry with %s", current.ObjectClass, entry.ObjectClass)}
	}
	return nil
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

func generatePassword() (string, error) {
	data := make([]byte, 24)
	if _, err := crand.Read(data); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(data), nil
}
