package directory

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/smarzola/ldaplite/internal/ldapdn"
	"github.com/smarzola/ldaplite/internal/models"
	"github.com/smarzola/ldaplite/internal/store"
	"github.com/smarzola/ldaplite/pkg/config"
	"github.com/smarzola/ldaplite/pkg/crypto"
)

var (
	ErrInvalidRequest      = errors.New("invalid directory request")
	ErrProtectedAttribute  = errors.New("protected attribute")
	ErrUnsupportedObject   = errors.New("unsupported object class")
	ErrPasswordNotProvided = errors.New("password is required")
)

type Service struct {
	store  store.Store
	cfg    *config.Config
	hasher *crypto.PasswordHasher
}

type UserInput struct {
	ParentDN   string              `json:"parentDN"`
	DN         string              `json:"dn"`
	UID        string              `json:"uid"`
	CN         string              `json:"cn"`
	SN         string              `json:"sn"`
	GivenName  string              `json:"givenName"`
	Mail       string              `json:"mail"`
	Password   string              `json:"password"`
	Attributes map[string][]string `json:"attributes"`
}

type GroupInput struct {
	ParentDN    string              `json:"parentDN"`
	DN          string              `json:"dn"`
	CN          string              `json:"cn"`
	Description string              `json:"description"`
	Members     []string            `json:"members"`
	Attributes  map[string][]string `json:"attributes"`
}

type OUInput struct {
	ParentDN    string              `json:"parentDN"`
	DN          string              `json:"dn"`
	OU          string              `json:"ou"`
	Description string              `json:"description"`
	Attributes  map[string][]string `json:"attributes"`
}

func NewService(st store.Store, cfg *config.Config) *Service {
	return &Service{
		store:  st,
		cfg:    cfg,
		hasher: crypto.NewPasswordHasher(cfg.Security.Argon2Config),
	}
}

func (s *Service) CreateUser(ctx context.Context, input UserInput) (*models.Entry, error) {
	parentDN := strings.TrimSpace(input.ParentDN)
	uid := strings.TrimSpace(input.UID)
	cn := strings.TrimSpace(input.CN)
	sn := strings.TrimSpace(input.SN)
	if parentDN == "" || uid == "" || cn == "" || sn == "" {
		return nil, fmt.Errorf("%w: parentDN, uid, cn, and sn are required", ErrInvalidRequest)
	}
	if strings.TrimSpace(input.Password) == "" {
		return nil, ErrPasswordNotProvided
	}

	user := models.NewUser(parentDN, uid, cn, sn, strings.TrimSpace(input.Mail))
	setOptional(user.Entry, "givenName", input.GivenName)
	if err := setProcessedPassword(s.hasher, user.Entry, input.Password); err != nil {
		return nil, err
	}
	if err := applyExtraAttributes(user.Entry, input.Attributes, userPreservedAttributes); err != nil {
		return nil, err
	}

	if err := s.store.CreateEntry(ctx, user.Entry); err != nil {
		return nil, err
	}
	return s.store.GetEntry(ctx, user.DN)
}

func (s *Service) UpdateUser(ctx context.Context, dn string, input UserInput) (*models.Entry, error) {
	entry, err := s.requireEntry(ctx, dn, models.ObjectClassInetOrgPerson)
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(input.CN) == "" || strings.TrimSpace(input.SN) == "" {
		return nil, fmt.Errorf("%w: cn and sn are required", ErrInvalidRequest)
	}
	entry.SetAttribute("cn", strings.TrimSpace(input.CN))
	entry.SetAttribute("sn", strings.TrimSpace(input.SN))
	setOptional(entry, "givenName", input.GivenName)
	setOptional(entry, "mail", input.Mail)
	if strings.TrimSpace(input.Password) != "" {
		if err := setProcessedPassword(s.hasher, entry, input.Password); err != nil {
			return nil, err
		}
	}
	if err := replaceExtraAttributes(entry, input.Attributes, userPreservedAttributes); err != nil {
		return nil, err
	}

	if err := s.store.UpdateEntry(ctx, entry); err != nil {
		return nil, err
	}
	return s.store.GetEntry(ctx, entry.DN)
}

func (s *Service) CreateGroup(ctx context.Context, input GroupInput) (*models.Entry, error) {
	parentDN := strings.TrimSpace(input.ParentDN)
	cn := strings.TrimSpace(input.CN)
	if parentDN == "" || cn == "" {
		return nil, fmt.Errorf("%w: parentDN and cn are required", ErrInvalidRequest)
	}
	members := cleanNonEmpty(input.Members)
	if len(members) == 0 {
		return nil, fmt.Errorf("%w: at least one group member is required", ErrInvalidRequest)
	}

	group := models.NewGroup(parentDN, cn, strings.TrimSpace(input.Description))
	for _, member := range members {
		group.AddMember(member)
	}
	if err := applyExtraAttributes(group.Entry, input.Attributes, groupPreservedAttributes); err != nil {
		return nil, err
	}

	if err := s.store.CreateEntry(ctx, group.Entry); err != nil {
		return nil, err
	}
	return s.store.GetEntry(ctx, group.DN)
}

func (s *Service) UpdateGroup(ctx context.Context, dn string, input GroupInput) (*models.Entry, error) {
	entry, err := s.requireEntry(ctx, dn, models.ObjectClassGroupOfNames)
	if err != nil {
		return nil, err
	}
	members := cleanNonEmpty(input.Members)
	if len(members) == 0 {
		return nil, fmt.Errorf("%w: at least one group member is required", ErrInvalidRequest)
	}

	setOptional(entry, "description", input.Description)
	entry.SetAttributes("member", members)
	if err := replaceExtraAttributes(entry, input.Attributes, groupPreservedAttributes); err != nil {
		return nil, err
	}

	if err := s.store.UpdateEntry(ctx, entry); err != nil {
		return nil, err
	}
	return s.store.GetEntry(ctx, entry.DN)
}

func (s *Service) CreateOU(ctx context.Context, input OUInput) (*models.Entry, error) {
	parentDN := strings.TrimSpace(input.ParentDN)
	ou := strings.TrimSpace(input.OU)
	if parentDN == "" || ou == "" {
		return nil, fmt.Errorf("%w: parentDN and ou are required", ErrInvalidRequest)
	}

	ouEntry := models.NewOrganizationalUnit(parentDN, ou, strings.TrimSpace(input.Description))
	if err := applyExtraAttributes(ouEntry.Entry, input.Attributes, ouPreservedAttributes); err != nil {
		return nil, err
	}
	if err := s.store.CreateEntry(ctx, ouEntry.Entry); err != nil {
		return nil, err
	}
	return s.store.GetEntry(ctx, ouEntry.DN)
}

func (s *Service) UpdateOU(ctx context.Context, dn string, input OUInput) (*models.Entry, error) {
	entry, err := s.requireEntry(ctx, dn, models.ObjectClassOrganizationalUnit)
	if err != nil {
		return nil, err
	}
	setOptional(entry, "description", input.Description)
	if err := replaceExtraAttributes(entry, input.Attributes, ouPreservedAttributes); err != nil {
		return nil, err
	}

	if err := s.store.UpdateEntry(ctx, entry); err != nil {
		return nil, err
	}
	return s.store.GetEntry(ctx, entry.DN)
}

func (s *Service) DeleteEntry(ctx context.Context, dn string) error {
	dn = strings.TrimSpace(dn)
	if dn == "" {
		return fmt.Errorf("%w: dn is required", ErrInvalidRequest)
	}
	return s.store.DeleteEntry(ctx, dn)
}

func (s *Service) ChangeOwnPassword(ctx context.Context, userDN, password string) error {
	if strings.TrimSpace(userDN) == "" {
		return fmt.Errorf("%w: authenticated user DN is required", ErrInvalidRequest)
	}
	return s.setUserPassword(ctx, userDN, password)
}

func (s *Service) ResetPassword(ctx context.Context, targetDN, password string) error {
	return s.setUserPassword(ctx, targetDN, password)
}

func (s *Service) setUserPassword(ctx context.Context, dn, password string) error {
	if strings.TrimSpace(password) == "" {
		return ErrPasswordNotProvided
	}
	entry, err := s.requireEntry(ctx, dn, models.ObjectClassInetOrgPerson)
	if err != nil {
		return err
	}
	if err := setProcessedPassword(s.hasher, entry, password); err != nil {
		return err
	}
	return s.store.UpdateEntry(ctx, entry)
}

func (s *Service) requireEntry(ctx context.Context, dn string, objectClass models.ObjectClass) (*models.Entry, error) {
	dn = strings.TrimSpace(dn)
	if dn == "" {
		return nil, fmt.Errorf("%w: dn is required", ErrInvalidRequest)
	}
	entry, err := s.store.GetEntryWithOptions(ctx, dn, store.EntryOptions{IncludeMemberOf: false})
	if err != nil {
		return nil, err
	}
	if entry == nil {
		return nil, fmt.Errorf("%w: %s", store.ErrNoSuchObject, dn)
	}
	if entry.ObjectClass != string(objectClass) {
		return nil, fmt.Errorf("%w: %s is %s", ErrUnsupportedObject, dn, entry.ObjectClass)
	}
	return entry, nil
}

func setProcessedPassword(hasher *crypto.PasswordHasher, entry *models.Entry, password string) error {
	processed, err := hasher.ProcessPassword(password)
	if err != nil {
		return err
	}
	entry.SetAttribute("userPassword", processed)
	return nil
}

func setOptional(entry *models.Entry, name, value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		entry.RemoveAttribute(name)
		return
	}
	entry.SetAttribute(name, value)
}

func applyExtraAttributes(entry *models.Entry, attrs map[string][]string, preserve map[string]struct{}) error {
	for name, values := range attrs {
		normalized := strings.ToLower(strings.TrimSpace(name))
		if normalized == "" {
			continue
		}
		if _, ok := preserve[normalized]; ok || isProtectedAttribute(normalized) {
			return fmt.Errorf("%w: %s", ErrProtectedAttribute, name)
		}
		entry.SetAttributes(normalized, cleanNonEmpty(values))
	}
	return nil
}

func replaceExtraAttributes(entry *models.Entry, attrs map[string][]string, preserve map[string]struct{}) error {
	for name := range entry.Attributes {
		normalized := strings.ToLower(name)
		if _, ok := preserve[normalized]; !ok {
			entry.RemoveAttribute(name)
		}
	}
	return applyExtraAttributes(entry, attrs, preserve)
}

func isProtectedAttribute(name string) bool {
	switch strings.ToLower(name) {
	case "objectclass", "userpassword", "createtimestamp", "modifytimestamp", "memberof", "entryuuid", "uuid":
		return true
	default:
		return false
	}
}

func cleanNonEmpty(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			result = append(result, value)
		}
	}
	return result
}

var userPreservedAttributes = toSet("uid", "cn", "sn", "givenname", "mail", "userpassword")
var groupPreservedAttributes = toSet("cn", "description", "member")
var ouPreservedAttributes = toSet("ou", "description")

func toSet(names ...string) map[string]struct{} {
	set := make(map[string]struct{}, len(names))
	for _, name := range names {
		set[strings.ToLower(name)] = struct{}{}
	}
	return set
}

func EqualDN(a, b string) bool {
	return ldapdn.Equal(a, b)
}
