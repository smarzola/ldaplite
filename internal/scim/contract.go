package scim

import "github.com/smarzola/ldaplite/internal/authz"

const (
	BasePath    = "/scim/v2"
	ContentType = "application/scim+json"

	errorSchema                 = "urn:ietf:params:scim:api:messages:2.0:Error"
	listResponseSchema          = "urn:ietf:params:scim:api:messages:2.0:ListResponse"
	serviceProviderConfigSchema = "urn:ietf:params:scim:schemas:core:2.0:ServiceProviderConfig"
	schemaSchema                = "urn:ietf:params:scim:schemas:core:2.0:Schema"
	resourceTypeSchema          = "urn:ietf:params:scim:schemas:core:2.0:ResourceType"
	userSchema                  = "urn:ietf:params:scim:schemas:core:2.0:User"
	groupSchema                 = "urn:ietf:params:scim:schemas:core:2.0:Group"
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
