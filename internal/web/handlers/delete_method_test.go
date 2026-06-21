package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDeleteHandlersRejectGet(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		handler http.HandlerFunc
	}{
		{
			name:    "users",
			path:    "/users/delete?dn=uid=test,ou=users,dc=example,dc=com",
			handler: (&UserHandler{}).Delete,
		},
		{
			name:    "groups",
			path:    "/groups/delete?dn=cn=test,ou=groups,dc=example,dc=com",
			handler: (&GroupHandler{}).Delete,
		},
		{
			name:    "ous",
			path:    "/ous/delete?dn=ou=test,dc=example,dc=com",
			handler: (&OUHandler{}).Delete,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rr := httptest.NewRecorder()

			tt.handler.ServeHTTP(rr, req)

			if rr.Code != http.StatusMethodNotAllowed {
				t.Fatalf("status = %d, want %d", rr.Code, http.StatusMethodNotAllowed)
			}
		})
	}
}
