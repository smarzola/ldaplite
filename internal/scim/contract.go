package scim

import "github.com/smarzola/ldaplite/internal/authz"

const (
	BasePath    = "/scim/v2"
	ContentType = "application/scim+json"

	errorSchema = "urn:ietf:params:scim:api:messages:2.0:Error"
)

type Contract struct {
	BasePath           string
	DiscoveryEndpoints []string
	ResourceTypes      []string
	UserFilters        []string
	GroupFilters       []string
	AuthScheme         string
	ReadCapability     string
	WriteCapability    string
	SupportsPatch      bool
	SupportsBulk       bool
}

func DefaultContract() Contract {
	return Contract{
		BasePath: BasePath,
		DiscoveryEndpoints: []string{
			BasePath + "/ServiceProviderConfig",
			BasePath + "/Schemas",
			BasePath + "/ResourceTypes",
		},
		ResourceTypes: []string{"User", "Group"},
		UserFilters: []string{
			`id eq "..."`,
			`userName eq "..."`,
			`displayName eq "..."`,
		},
		GroupFilters: []string{
			`id eq "..."`,
			`displayName eq "..."`,
		},
		AuthScheme:      "HTTP Basic with LDAPLite user credentials",
		ReadCapability:  string(authz.DirectoryRead),
		WriteCapability: string(authz.DirectoryWrite),
		SupportsPatch:   false,
		SupportsBulk:    false,
	}
}
