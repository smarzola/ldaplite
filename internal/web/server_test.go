package web

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/smarzola/ldaplite/internal/models"
	"github.com/smarzola/ldaplite/internal/store"
	"github.com/smarzola/ldaplite/pkg/config"
	"github.com/smarzola/ldaplite/pkg/crypto"
)

func TestMutatingRoutesRejectCrossOriginWithValidAuth(t *testing.T) {
	srv, st := setupTestServer(t)
	defer st.Close()

	tests := []struct {
		name string
		path string
		form string
	}{
		{
			name: "users",
			path: "/users/new",
			form: "parentDN=ou%3Dusers%2Cdc%3Dtest%2Cdc%3Dcom&uid=jdoe&cn=John+Doe&sn=Doe&userPassword=Secret123%21",
		},
		{
			name: "groups",
			path: "/groups/new",
			form: "parentDN=ou%3Dgroups%2Cdc%3Dtest%2Cdc%3Dcom&cn=developers&member=uid%3Dadmin%2Cou%3Dusers%2Cdc%3Dtest%2Cdc%3Dcom",
		},
		{
			name: "ous",
			path: "/ous/new",
			form: "parentDN=dc%3Dtest%2Cdc%3Dcom&ou=engineering",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "http://ldaplite.test"+tt.path, strings.NewReader(tt.form))
			req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("admin:TestPassword123!")))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			req.Header.Set("Origin", "https://evil.test")
			rr := httptest.NewRecorder()

			srv.mux.ServeHTTP(rr, req)

			if rr.Code != http.StatusForbidden {
				t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusForbidden, rr.Body.String())
			}
		})
	}
}

func TestRootRedirectsToEmbeddedApp(t *testing.T) {
	srv, st := setupTestServer(t)
	defer st.Close()

	req := httptest.NewRequest(http.MethodGet, "http://ldaplite.test/", nil)
	rr := httptest.NewRecorder()

	srv.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusFound)
	}
	if got := rr.Header().Get("Location"); got != "/app/" {
		t.Fatalf("Location = %q, want /app/", got)
	}
}

func TestAPISessionReturnsRoleAwareCapabilities(t *testing.T) {
	srv, st := setupTestServer(t)
	defer st.Close()

	createTestUser(t, st, "regularuser", "RegularPassword123!")
	createTestUser(t, st, "passworduser", "PasswordOnly123!")
	createTestGroup(t, st, "ldaplite.password", "uid=passworduser,ou=users,dc=test,dc=com")

	tests := []struct {
		name               string
		credentials        string
		wantAdmin          bool
		wantDirectoryRead  bool
		wantDirectoryWrite bool
	}{
		{
			name:               "admin",
			credentials:        "admin:TestPassword123!",
			wantAdmin:          true,
			wantDirectoryRead:  true,
			wantDirectoryWrite: true,
		},
		{
			name:               "authenticated non-admin",
			credentials:        "regularuser:RegularPassword123!",
			wantAdmin:          false,
			wantDirectoryRead:  true,
			wantDirectoryWrite: false,
		},
		{
			name:               "password-only user",
			credentials:        "passworduser:PasswordOnly123!",
			wantAdmin:          false,
			wantDirectoryRead:  false,
			wantDirectoryWrite: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "http://ldaplite.test/api/session", nil)
			req.Header.Set("Authorization", basicAuth(tt.credentials))
			rr := httptest.NewRecorder()

			srv.mux.ServeHTTP(rr, req)

			if rr.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusOK, rr.Body.String())
			}

			var got struct {
				UserDN       string   `json:"userDN"`
				Capabilities []string `json:"capabilities"`
				Roles        struct {
					Admin          bool `json:"admin"`
					DirectoryRead  bool `json:"directoryRead"`
					DirectoryWrite bool `json:"directoryWrite"`
					PasswordSelf   bool `json:"passwordSelf"`
				} `json:"roles"`
			}
			if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
				t.Fatalf("failed to decode session response: %v", err)
			}

			if got.UserDN == "" {
				t.Fatal("userDN was empty")
			}
			if got.Roles.Admin != tt.wantAdmin {
				t.Fatalf("roles.admin = %v, want %v", got.Roles.Admin, tt.wantAdmin)
			}
			if got.Roles.DirectoryRead != tt.wantDirectoryRead {
				t.Fatalf("roles.directoryRead = %v, want %v", got.Roles.DirectoryRead, tt.wantDirectoryRead)
			}
			if got.Roles.DirectoryWrite != tt.wantDirectoryWrite {
				t.Fatalf("roles.directoryWrite = %v, want %v", got.Roles.DirectoryWrite, tt.wantDirectoryWrite)
			}
			if got.Roles.PasswordSelf != true {
				t.Fatal("roles.passwordSelf = false, want true")
			}
			if !containsString(got.Capabilities, "ui.read") {
				t.Fatalf("capabilities missing ui.read: %v", got.Capabilities)
			}
			if containsString(got.Capabilities, "ui.admin") != tt.wantAdmin {
				t.Fatalf("ui.admin presence = %v, want %v in %v", containsString(got.Capabilities, "ui.admin"), tt.wantAdmin, got.Capabilities)
			}
		})
	}
}

func TestPasswordOnlyUserCanChangePasswordButCannotReadDirectoryAPI(t *testing.T) {
	srv, st := setupTestServer(t)
	defer st.Close()

	createTestUser(t, st, "passworduser", "PasswordOnly123!")
	createTestGroup(t, st, "ldaplite.password", "uid=passworduser,ou=users,dc=test,dc=com")

	readReq := httptest.NewRequest(http.MethodGet, "http://ldaplite.test/api/directory", nil)
	readReq.Header.Set("Authorization", basicAuth("passworduser:PasswordOnly123!"))
	readRR := httptest.NewRecorder()

	srv.mux.ServeHTTP(readRR, readReq)

	if readRR.Code != http.StatusForbidden {
		t.Fatalf("directory status = %d, want %d; body=%s", readRR.Code, http.StatusForbidden, readRR.Body.String())
	}

	changeReq := apiJSONRequest(t, http.MethodPost, "/api/account/password", "passworduser:PasswordOnly123!", map[string]any{
		"password": "PasswordOnlyChanged123!",
	})
	changeReq.Header.Set("Origin", "http://ldaplite.test")
	changeRR := httptest.NewRecorder()

	srv.mux.ServeHTTP(changeRR, changeReq)

	if changeRR.Code != http.StatusNoContent {
		t.Fatalf("change password status = %d, want %d; body=%s", changeRR.Code, http.StatusNoContent, changeRR.Body.String())
	}
	assertPasswordValid(t, st, "passworduser", "PasswordOnlyChanged123!")
}

func TestNonAdminCanReadDirectoryAPIButCannotReachWriteRoute(t *testing.T) {
	srv, st := setupTestServer(t)
	defer st.Close()

	createTestUser(t, st, "regularuser", "RegularPassword123!")

	readReq := httptest.NewRequest(http.MethodGet, "http://ldaplite.test/api/directory", nil)
	readReq.Header.Set("Authorization", basicAuth("regularuser:RegularPassword123!"))
	readRR := httptest.NewRecorder()

	srv.mux.ServeHTTP(readRR, readReq)

	if readRR.Code != http.StatusOK {
		t.Fatalf("directory status = %d, want %d; body=%s", readRR.Code, http.StatusOK, readRR.Body.String())
	}

	var directory struct {
		BaseDN string `json:"baseDN"`
		Users  []struct {
			DN   string `json:"dn"`
			Name string `json:"name"`
		} `json:"users"`
	}
	if err := json.Unmarshal(readRR.Body.Bytes(), &directory); err != nil {
		t.Fatalf("failed to decode directory response: %v", err)
	}
	if directory.BaseDN != "dc=test,dc=com" {
		t.Fatalf("baseDN = %q, want dc=test,dc=com", directory.BaseDN)
	}
	if !directoryHasUser(directory.Users, "regularuser") {
		t.Fatalf("directory response missing regularuser: %+v", directory.Users)
	}

	writeReq := httptest.NewRequest(http.MethodPost, "http://ldaplite.test/users/new", strings.NewReader("parentDN=ou%3Dusers%2Cdc%3Dtest%2Cdc%3Dcom&uid=hacker&cn=Hack&sn=Er&userPassword=Secret123%21"))
	writeReq.Header.Set("Authorization", basicAuth("regularuser:RegularPassword123!"))
	writeReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	writeReq.Header.Set("Origin", "http://ldaplite.test")
	writeRR := httptest.NewRecorder()

	srv.mux.ServeHTTP(writeRR, writeReq)

	if writeRR.Code != http.StatusForbidden {
		t.Fatalf("write status = %d, want %d; body=%s", writeRR.Code, http.StatusForbidden, writeRR.Body.String())
	}
}

func TestDirectorySearchAPISupportsQueryTypeFilterAndPagination(t *testing.T) {
	srv, st := setupTestServer(t)
	defer st.Close()

	createTestUser(t, st, "regularuser", "RegularPassword123!")
	createTestUserWithAttrs(t, st, "anna", "AnnaPassword123!", map[string][]string{
		"cn":          {"Anna Operator"},
		"sn":          {"Operator"},
		"mail":        {"anna@example.com"},
		"description": {"Directory operator"},
	})
	createTestUserWithAttrs(t, st, "brian", "BrianPassword123!", map[string][]string{
		"cn":   {"Brian Builder"},
		"sn":   {"Builder"},
		"mail": {"brian@example.com"},
	})
	createTestGroup(t, st, "ops-team", "uid=anna,ou=users,dc=test,dc=com")
	createTestOU(t, st, "engineering", "Engineering directory entries")

	searchReq := httptest.NewRequest(http.MethodGet, "http://ldaplite.test/api/directory/search?type=users&q=anna%40example.com&page=1&pageSize=10", nil)
	searchReq.Header.Set("Authorization", basicAuth("regularuser:RegularPassword123!"))
	searchRR := httptest.NewRecorder()

	srv.mux.ServeHTTP(searchRR, searchReq)

	if searchRR.Code != http.StatusOK {
		t.Fatalf("search status = %d, want %d; body=%s", searchRR.Code, http.StatusOK, searchRR.Body.String())
	}
	var search directorySearchTestResponse
	if err := json.Unmarshal(searchRR.Body.Bytes(), &search); err != nil {
		t.Fatalf("failed to decode search response: %v", err)
	}
	if search.BaseDN != "dc=test,dc=com" {
		t.Fatalf("baseDN = %q, want dc=test,dc=com", search.BaseDN)
	}
	if search.Type != "users" || search.Query != "anna@example.com" {
		t.Fatalf("search echo = type %q query %q, want users anna@example.com", search.Type, search.Query)
	}
	if search.Total != 1 || len(search.Entries) != 1 {
		t.Fatalf("search entries = total %d len %d, want 1/1: %+v", search.Total, len(search.Entries), search.Entries)
	}
	if got := search.Entries[0]; got.Name != "anna" || got.Type != "user" || got.Mail != "anna@example.com" {
		t.Fatalf("entry = %+v, want anna user with mail", got)
	}

	groupReq := httptest.NewRequest(http.MethodGet, "http://ldaplite.test/api/directory/search?type=groups&q=ops-team", nil)
	groupReq.Header.Set("Authorization", basicAuth("regularuser:RegularPassword123!"))
	groupRR := httptest.NewRecorder()

	srv.mux.ServeHTTP(groupRR, groupReq)

	if groupRR.Code != http.StatusOK {
		t.Fatalf("group search status = %d, want %d; body=%s", groupRR.Code, http.StatusOK, groupRR.Body.String())
	}
	var groupSearch directorySearchTestResponse
	if err := json.Unmarshal(groupRR.Body.Bytes(), &groupSearch); err != nil {
		t.Fatalf("failed to decode group search response: %v", err)
	}
	if groupSearch.Total != 1 || groupSearch.Entries[0].Name != "ops-team" || groupSearch.Entries[0].Type != "group" {
		t.Fatalf("group search = %+v, want ops-team group", groupSearch)
	}

	ouReq := httptest.NewRequest(http.MethodGet, "http://ldaplite.test/api/directory/search?type=ous&q=engineering", nil)
	ouReq.Header.Set("Authorization", basicAuth("regularuser:RegularPassword123!"))
	ouRR := httptest.NewRecorder()

	srv.mux.ServeHTTP(ouRR, ouReq)

	if ouRR.Code != http.StatusOK {
		t.Fatalf("ou search status = %d, want %d; body=%s", ouRR.Code, http.StatusOK, ouRR.Body.String())
	}
	var ouSearch directorySearchTestResponse
	if err := json.Unmarshal(ouRR.Body.Bytes(), &ouSearch); err != nil {
		t.Fatalf("failed to decode ou search response: %v", err)
	}
	if ouSearch.Total != 1 || ouSearch.Entries[0].Name != "engineering" || ouSearch.Entries[0].Type != "ou" {
		t.Fatalf("ou search = %+v, want engineering ou", ouSearch)
	}

	pageOneReq := httptest.NewRequest(http.MethodGet, "http://ldaplite.test/api/directory/search?type=users&page=1&pageSize=1", nil)
	pageOneReq.Header.Set("Authorization", basicAuth("regularuser:RegularPassword123!"))
	pageOneRR := httptest.NewRecorder()

	srv.mux.ServeHTTP(pageOneRR, pageOneReq)

	if pageOneRR.Code != http.StatusOK {
		t.Fatalf("page one status = %d, want %d; body=%s", pageOneRR.Code, http.StatusOK, pageOneRR.Body.String())
	}
	var pageOne directorySearchTestResponse
	if err := json.Unmarshal(pageOneRR.Body.Bytes(), &pageOne); err != nil {
		t.Fatalf("failed to decode page one response: %v", err)
	}
	if pageOne.Page != 1 || pageOne.PageSize != 1 || pageOne.TotalPages < 2 || len(pageOne.Entries) != 1 {
		t.Fatalf("page one metadata = %+v, want one result and at least two pages", pageOne)
	}

	pageTwoReq := httptest.NewRequest(http.MethodGet, "http://ldaplite.test/api/directory/search?type=users&page=2&pageSize=1", nil)
	pageTwoReq.Header.Set("Authorization", basicAuth("regularuser:RegularPassword123!"))
	pageTwoRR := httptest.NewRecorder()

	srv.mux.ServeHTTP(pageTwoRR, pageTwoReq)

	if pageTwoRR.Code != http.StatusOK {
		t.Fatalf("page two status = %d, want %d; body=%s", pageTwoRR.Code, http.StatusOK, pageTwoRR.Body.String())
	}
	var pageTwo directorySearchTestResponse
	if err := json.Unmarshal(pageTwoRR.Body.Bytes(), &pageTwo); err != nil {
		t.Fatalf("failed to decode page two response: %v", err)
	}
	if len(pageTwo.Entries) != 1 || pageTwo.Entries[0].DN == pageOne.Entries[0].DN {
		t.Fatalf("pagination was not deterministic: page1=%+v page2=%+v", pageOne.Entries, pageTwo.Entries)
	}
}

func TestDirectoryDetailAPIReturnsSafeAttributesAndMemberships(t *testing.T) {
	srv, st := setupTestServer(t)
	defer st.Close()

	createTestUser(t, st, "regularuser", "RegularPassword123!")
	createTestUserWithAttrs(t, st, "detailuser", "DetailPassword123!", map[string][]string{
		"cn":          {"Detail User"},
		"sn":          {"User"},
		"mail":        {"detail@example.com"},
		"description": {"Visible directory detail"},
	})
	createTestGroup(t, st, "detail-readers", "uid=detailuser,ou=users,dc=test,dc=com")

	req := httptest.NewRequest(http.MethodGet, "http://ldaplite.test/api/directory/entry?dn="+url.QueryEscape("uid=detailuser,ou=users,dc=test,dc=com"), nil)
	req.Header.Set("Authorization", basicAuth("regularuser:RegularPassword123!"))
	rr := httptest.NewRecorder()

	srv.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("detail status = %d, want %d; body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}
	if strings.Contains(strings.ToLower(rr.Body.String()), "userpassword") || strings.Contains(rr.Body.String(), "DetailPassword123") || strings.Contains(rr.Body.String(), "ARGON2") {
		t.Fatalf("detail API leaked password material: %s", rr.Body.String())
	}

	var detail struct {
		BaseDN string `json:"baseDN"`
		Entry  struct {
			DN          string              `json:"dn"`
			Type        string              `json:"type"`
			Name        string              `json:"name"`
			Mail        string              `json:"mail"`
			MemberOf    []string            `json:"memberOf"`
			Attributes  map[string][]string `json:"attributes"`
			ObjectClass string              `json:"objectClass"`
		} `json:"entry"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &detail); err != nil {
		t.Fatalf("failed to decode detail response: %v", err)
	}
	if detail.BaseDN != "dc=test,dc=com" {
		t.Fatalf("baseDN = %q, want dc=test,dc=com", detail.BaseDN)
	}
	if detail.Entry.DN != "uid=detailuser,ou=users,dc=test,dc=com" || detail.Entry.Type != "user" || detail.Entry.Name != "detailuser" {
		t.Fatalf("entry identity = %+v, want detailuser user", detail.Entry)
	}
	if detail.Entry.Mail != "detail@example.com" || detail.Entry.ObjectClass != "inetOrgPerson" {
		t.Fatalf("entry summary = %+v, want mail and inetOrgPerson", detail.Entry)
	}
	if !containsString(detail.Entry.MemberOf, "cn=detail-readers,ou=groups,dc=test,dc=com") {
		t.Fatalf("memberOf missing detail-readers: %+v", detail.Entry.MemberOf)
	}
	if _, ok := detail.Entry.Attributes["userpassword"]; ok {
		t.Fatalf("safe attributes included userpassword: %+v", detail.Entry.Attributes)
	}
	if got := detail.Entry.Attributes["mail"]; len(got) != 1 || got[0] != "detail@example.com" {
		t.Fatalf("attributes[mail] = %+v, want detail@example.com", got)
	}
}

func TestPasswordOnlyUserCannotReachDirectorySearchOrDetailAPIs(t *testing.T) {
	srv, st := setupTestServer(t)
	defer st.Close()

	createTestUser(t, st, "passworduser", "PasswordOnly123!")
	createTestGroup(t, st, "ldaplite.password", "uid=passworduser,ou=users,dc=test,dc=com")

	tests := []struct {
		name string
		path string
	}{
		{
			name: "search",
			path: "/api/directory/search?q=admin",
		},
		{
			name: "detail",
			path: "/api/directory/entry?dn=" + url.QueryEscape("uid=admin,ou=users,dc=test,dc=com"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "http://ldaplite.test"+tt.path, nil)
			req.Header.Set("Authorization", basicAuth("passworduser:PasswordOnly123!"))
			rr := httptest.NewRecorder()

			srv.mux.ServeHTTP(rr, req)

			if rr.Code != http.StatusForbidden {
				t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusForbidden, rr.Body.String())
			}
		})
	}
}

func TestAdminDirectoryWriteAPI(t *testing.T) {
	srv, st := setupTestServer(t)
	defer st.Close()

	userPayload := map[string]any{
		"parentDN": "ou=users,dc=test,dc=com",
		"uid":      "apiuser",
		"cn":       "API User",
		"sn":       "User",
		"mail":     "apiuser@example.com",
		"password": "Secret123!",
		"attributes": map[string][]string{
			"telephoneNumber": {"123"},
		},
	}
	createUser := apiJSONRequest(t, http.MethodPost, "/api/users", "admin:TestPassword123!", userPayload)
	createUser.Header.Set("Origin", "http://ldaplite.test")
	createUserRR := httptest.NewRecorder()

	srv.mux.ServeHTTP(createUserRR, createUser)

	if createUserRR.Code != http.StatusCreated {
		t.Fatalf("create user status = %d, want %d; body=%s", createUserRR.Code, http.StatusCreated, createUserRR.Body.String())
	}
	if strings.Contains(createUserRR.Body.String(), "Secret123") || strings.Contains(createUserRR.Body.String(), "ARGON2") {
		t.Fatalf("create user response leaked password material: %s", createUserRR.Body.String())
	}
	assertPasswordValid(t, st, "apiuser", "Secret123!")

	userDN := "uid=apiuser,ou=users,dc=test,dc=com"
	updateUserPayload := map[string]any{
		"cn":   "API User Renamed",
		"sn":   "Renamed",
		"mail": "renamed@example.com",
		"attributes": map[string][]string{
			"description": {"managed through api"},
		},
	}
	updateUser := apiJSONRequest(t, http.MethodPut, "/api/users?dn="+url.QueryEscape(userDN), "admin:TestPassword123!", updateUserPayload)
	updateUser.Header.Set("Origin", "http://ldaplite.test")
	updateUserRR := httptest.NewRecorder()

	srv.mux.ServeHTTP(updateUserRR, updateUser)

	if updateUserRR.Code != http.StatusOK {
		t.Fatalf("update user status = %d, want %d; body=%s", updateUserRR.Code, http.StatusOK, updateUserRR.Body.String())
	}
	updatedUser, err := st.GetEntry(context.Background(), userDN)
	if err != nil {
		t.Fatalf("GetEntry(updated user) failed: %v", err)
	}
	if got := updatedUser.GetAttribute("description"); got != "managed through api" {
		t.Fatalf("description = %q, want managed through api", got)
	}

	groupPayload := map[string]any{
		"parentDN":    "ou=groups,dc=test,dc=com",
		"cn":          "api-editors",
		"description": "API editors",
		"members":     []string{userDN},
	}
	createGroup := apiJSONRequest(t, http.MethodPost, "/api/groups", "admin:TestPassword123!", groupPayload)
	createGroup.Header.Set("Origin", "http://ldaplite.test")
	createGroupRR := httptest.NewRecorder()

	srv.mux.ServeHTTP(createGroupRR, createGroup)

	if createGroupRR.Code != http.StatusCreated {
		t.Fatalf("create group status = %d, want %d; body=%s", createGroupRR.Code, http.StatusCreated, createGroupRR.Body.String())
	}

	groupDN := "cn=api-editors,ou=groups,dc=test,dc=com"
	badGroupUpdatePayload := map[string]any{
		"description": "bad update",
		"members":     []string{"uid=missing,ou=users,dc=test,dc=com"},
	}
	badGroupUpdate := apiJSONRequest(t, http.MethodPut, "/api/groups?dn="+url.QueryEscape(groupDN), "admin:TestPassword123!", badGroupUpdatePayload)
	badGroupUpdate.Header.Set("Origin", "http://ldaplite.test")
	badGroupUpdateRR := httptest.NewRecorder()

	srv.mux.ServeHTTP(badGroupUpdateRR, badGroupUpdate)

	if badGroupUpdateRR.Code != http.StatusBadRequest {
		t.Fatalf("bad group update status = %d, want %d; body=%s", badGroupUpdateRR.Code, http.StatusBadRequest, badGroupUpdateRR.Body.String())
	}
	groupAfterFailedUpdate, err := st.GetEntry(context.Background(), groupDN)
	if err != nil {
		t.Fatalf("GetEntry(group after failed update) failed: %v", err)
	}
	if !containsString(groupAfterFailedUpdate.GetAttributes("member"), userDN) {
		t.Fatalf("group member rollback failed, members=%v", groupAfterFailedUpdate.GetAttributes("member"))
	}

	ouPayload := map[string]any{
		"parentDN":    "dc=test,dc=com",
		"ou":          "api",
		"description": "API managed",
	}
	createOU := apiJSONRequest(t, http.MethodPost, "/api/ous", "admin:TestPassword123!", ouPayload)
	createOU.Header.Set("Origin", "http://ldaplite.test")
	createOURR := httptest.NewRecorder()

	srv.mux.ServeHTTP(createOURR, createOU)

	if createOURR.Code != http.StatusCreated {
		t.Fatalf("create ou status = %d, want %d; body=%s", createOURR.Code, http.StatusCreated, createOURR.Body.String())
	}
}

func TestPasswordAPIsAndDeniedDirectWrites(t *testing.T) {
	srv, st := setupTestServer(t)
	defer st.Close()

	createTestUser(t, st, "regularuser", "RegularPassword123!")
	createTestUser(t, st, "targetuser", "TargetPassword123!")

	selfChange := apiJSONRequest(t, http.MethodPost, "/api/account/password", "regularuser:RegularPassword123!", map[string]any{
		"password": "ChangedPassword123!",
	})
	selfChange.Header.Set("Origin", "http://ldaplite.test")
	selfChangeRR := httptest.NewRecorder()

	srv.mux.ServeHTTP(selfChangeRR, selfChange)

	if selfChangeRR.Code != http.StatusNoContent {
		t.Fatalf("self password status = %d, want %d; body=%s", selfChangeRR.Code, http.StatusNoContent, selfChangeRR.Body.String())
	}
	assertPasswordValid(t, st, "regularuser", "ChangedPassword123!")

	deniedReset := apiJSONRequest(t, http.MethodPost, "/api/users/password", "regularuser:ChangedPassword123!", map[string]any{
		"dn":       "uid=targetuser,ou=users,dc=test,dc=com",
		"password": "HackedPassword123!",
	})
	deniedReset.Header.Set("Origin", "http://ldaplite.test")
	deniedResetRR := httptest.NewRecorder()

	srv.mux.ServeHTTP(deniedResetRR, deniedReset)

	if deniedResetRR.Code != http.StatusForbidden {
		t.Fatalf("denied reset status = %d, want %d; body=%s", deniedResetRR.Code, http.StatusForbidden, deniedResetRR.Body.String())
	}
	assertPasswordValid(t, st, "targetuser", "TargetPassword123!")

	adminReset := apiJSONRequest(t, http.MethodPost, "/api/users/password", "admin:TestPassword123!", map[string]any{
		"dn":       "uid=targetuser,ou=users,dc=test,dc=com",
		"password": "ResetPassword123!",
	})
	adminReset.Header.Set("Origin", "http://ldaplite.test")
	adminResetRR := httptest.NewRecorder()

	srv.mux.ServeHTTP(adminResetRR, adminReset)

	if adminResetRR.Code != http.StatusNoContent {
		t.Fatalf("admin reset status = %d, want %d; body=%s", adminResetRR.Code, http.StatusNoContent, adminResetRR.Body.String())
	}
	assertPasswordValid(t, st, "targetuser", "ResetPassword123!")

	nonAdminCreate := apiJSONRequest(t, http.MethodPost, "/api/users", "regularuser:ChangedPassword123!", map[string]any{
		"parentDN": "ou=users,dc=test,dc=com",
		"uid":      "blocked",
		"cn":       "Blocked",
		"sn":       "Blocked",
		"password": "BlockedPassword123!",
	})
	nonAdminCreate.Header.Set("Origin", "http://ldaplite.test")
	nonAdminCreateRR := httptest.NewRecorder()

	srv.mux.ServeHTTP(nonAdminCreateRR, nonAdminCreate)

	if nonAdminCreateRR.Code != http.StatusForbidden {
		t.Fatalf("non-admin create status = %d, want %d; body=%s", nonAdminCreateRR.Code, http.StatusForbidden, nonAdminCreateRR.Body.String())
	}
}

func TestWriteAPIRejectsProtectedAttributesAndDoesNotExposePasswords(t *testing.T) {
	srv, st := setupTestServer(t)
	defer st.Close()

	protectedPayload := map[string]any{
		"parentDN": "ou=users,dc=test,dc=com",
		"uid":      "badattrs",
		"cn":       "Bad Attrs",
		"sn":       "Attrs",
		"password": "Secret123!",
		"attributes": map[string][]string{
			"userPassword": {"plaintext"},
		},
	}
	req := apiJSONRequest(t, http.MethodPost, "/api/users", "admin:TestPassword123!", protectedPayload)
	req.Header.Set("Origin", "http://ldaplite.test")
	rr := httptest.NewRecorder()

	srv.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("protected attr status = %d, want %d; body=%s", rr.Code, http.StatusBadRequest, rr.Body.String())
	}
	if hash, _, err := st.GetUserPasswordHash(context.Background(), "badattrs"); err != nil {
		t.Fatalf("GetUserPasswordHash() failed: %v", err)
	} else if hash != "" {
		t.Fatal("protected-attribute rejected user should not have been created")
	}

	createTestUser(t, st, "visibleuser", "VisiblePassword123!")
	directoryReq := httptest.NewRequest(http.MethodGet, "http://ldaplite.test/api/directory", nil)
	directoryReq.Header.Set("Authorization", basicAuth("admin:TestPassword123!"))
	directoryRR := httptest.NewRecorder()

	srv.mux.ServeHTTP(directoryRR, directoryReq)

	if directoryRR.Code != http.StatusOK {
		t.Fatalf("directory status = %d, want %d; body=%s", directoryRR.Code, http.StatusOK, directoryRR.Body.String())
	}
	if strings.Contains(directoryRR.Body.String(), "VisiblePassword123") || strings.Contains(directoryRR.Body.String(), "ARGON2") {
		t.Fatalf("directory API leaked password material: %s", directoryRR.Body.String())
	}
}

func setupTestServer(t *testing.T) (*Server, store.Store) {
	t.Helper()

	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})))
	t.Setenv("LDAP_ADMIN_PASSWORD", "TestPassword123!")

	cfg := &config.Config{
		LDAP: config.LDAPConfig{
			BaseDN: "dc=test,dc=com",
		},
		Database: config.DatabaseConfig{
			Path:            filepath.Join(t.TempDir(), "test.db"),
			MaxOpenConns:    5,
			MaxIdleConns:    2,
			ConnMaxLifetime: 300,
		},
		Security: config.SecurityConfig{
			Argon2Config: config.Argon2Config{
				Memory:      64,
				Iterations:  1,
				Parallelism: 1,
				SaltLength:  16,
				KeyLength:   32,
			},
		},
		WebUI: config.WebUIConfig{
			Enabled:     true,
			Port:        8080,
			BindAddress: "127.0.0.1",
		},
	}

	st := store.NewSQLiteStore(cfg)
	if err := st.Initialize(context.Background()); err != nil {
		t.Fatalf("Initialize() failed: %v", err)
	}

	srv, err := NewServer(cfg, st)
	if err != nil {
		st.Close()
		t.Fatalf("NewServer() failed: %v", err)
	}
	return srv, st
}

func basicAuth(credentials string) string {
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(credentials))
}

func createTestUser(t *testing.T, st store.Store, uid, password string) {
	t.Helper()
	createTestUserWithAttrs(t, st, uid, password, nil)
}

func createTestUserWithAttrs(t *testing.T, st store.Store, uid, password string, attrs map[string][]string) {
	t.Helper()

	hasher := crypto.NewPasswordHasher(config.Argon2Config{
		Memory:      64,
		Iterations:  1,
		Parallelism: 1,
		SaltLength:  16,
		KeyLength:   32,
	})
	hashedPassword, err := hasher.Hash(password)
	if err != nil {
		t.Fatalf("Hash() failed: %v", err)
	}

	user := models.NewEntry("uid="+uid+",ou=users,dc=test,dc=com", "inetOrgPerson")
	user.SetAttribute("uid", uid)
	user.SetAttribute("cn", uid)
	user.SetAttribute("sn", uid)
	user.SetAttribute("userPassword", hashedPassword)
	for name, values := range attrs {
		user.SetAttributes(name, values)
	}

	if err := st.CreateEntry(context.Background(), user); err != nil {
		t.Fatalf("CreateEntry(%s) failed: %v", uid, err)
	}
}

func createTestGroup(t *testing.T, st store.Store, cn string, members ...string) {
	t.Helper()

	group := models.NewEntry("cn="+cn+",ou=groups,dc=test,dc=com", "groupOfNames")
	group.SetAttribute("cn", cn)
	group.SetAttributes("member", members)

	if err := st.CreateEntry(context.Background(), group); err != nil {
		t.Fatalf("CreateEntry(%s) failed: %v", cn, err)
	}
}

func createTestOU(t *testing.T, st store.Store, ou, description string) {
	t.Helper()

	entry := models.NewEntry("ou="+ou+",dc=test,dc=com", "organizationalUnit")
	entry.SetAttribute("ou", ou)
	if description != "" {
		entry.SetAttribute("description", description)
	}

	if err := st.CreateEntry(context.Background(), entry); err != nil {
		t.Fatalf("CreateEntry(%s) failed: %v", ou, err)
	}
}

func apiJSONRequest(t *testing.T, method, target, credentials string, payload any) *http.Request {
	t.Helper()

	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("Marshal(%T) failed: %v", payload, err)
	}
	req := httptest.NewRequest(method, "http://ldaplite.test"+target, bytes.NewReader(body))
	req.Header.Set("Authorization", basicAuth(credentials))
	req.Header.Set("Content-Type", "application/json")
	return req
}

func assertPasswordValid(t *testing.T, st store.Store, uid, password string) {
	t.Helper()

	hash, _, err := st.GetUserPasswordHash(context.Background(), uid)
	if err != nil {
		t.Fatalf("GetUserPasswordHash(%s) failed: %v", uid, err)
	}
	hasher := crypto.NewPasswordHasher(config.Argon2Config{
		Memory:      64,
		Iterations:  1,
		Parallelism: 1,
		SaltLength:  16,
		KeyLength:   32,
	})
	valid, err := hasher.Verify(password, hash)
	if err != nil {
		t.Fatalf("Verify(%s) failed: %v", uid, err)
	}
	if !valid {
		t.Fatalf("password for %s was not updated to expected value", uid)
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func directoryHasUser(users []struct {
	DN   string `json:"dn"`
	Name string `json:"name"`
}, uid string) bool {
	for _, user := range users {
		if user.Name == uid {
			return true
		}
	}
	return false
}

type directorySearchTestResponse struct {
	BaseDN     string `json:"baseDN"`
	Query      string `json:"query"`
	Type       string `json:"type"`
	Page       int    `json:"page"`
	PageSize   int    `json:"pageSize"`
	Total      int    `json:"total"`
	TotalPages int    `json:"totalPages"`
	Entries    []struct {
		DN     string   `json:"dn"`
		Type   string   `json:"type"`
		Name   string   `json:"name"`
		Mail   string   `json:"mail"`
		Member []string `json:"members"`
	} `json:"entries"`
}
