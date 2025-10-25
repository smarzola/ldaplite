package schema

import (
	"strings"
	"testing"
)

func TestCompileEquality(t *testing.T) {
	compiler := NewFilterCompiler()

	tests := []struct {
		name        string
		filter      *Filter
		wantSQL     string
		wantArgsLen int
		wantErr     bool
	}{
		{
			name: "objectClass equality",
			filter: &Filter{
				Type:      FilterTypeEquality,
				Attribute: "objectClass",
				Value:     "inetOrgPerson",
			},
			wantSQL:     "LOWER(e.object_class) = LOWER(?)",
			wantArgsLen: 1,
		},
		{
			name: "attribute equality",
			filter: &Filter{
				Type:      FilterTypeEquality,
				Attribute: "uid",
				Value:     "jdoe",
			},
			wantSQL:     "EXISTS",
			wantArgsLen: 2,
		},
		{
			name: "cn equality",
			filter: &Filter{
				Type:      FilterTypeEquality,
				Attribute: "cn",
				Value:     "John Doe",
			},
			wantSQL:     "EXISTS",
			wantArgsLen: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sql, args, err := compiler.CompileToSQL(tt.filter)

			if (err != nil) != tt.wantErr {
				t.Errorf("CompileToSQL() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !strings.Contains(sql, tt.wantSQL) {
				t.Errorf("CompileToSQL() SQL = %v, want to contain %v", sql, tt.wantSQL)
			}

			if len(args) != tt.wantArgsLen {
				t.Errorf("CompileToSQL() args length = %v, want %v", len(args), tt.wantArgsLen)
			}
		})
	}
}

func TestCompilePresent(t *testing.T) {
	compiler := NewFilterCompiler()

	tests := []struct {
		name        string
		filter      *Filter
		wantSQL     string
		wantArgsLen int
	}{
		{
			name: "objectClass present",
			filter: &Filter{
				Type:      FilterTypePresent,
				Attribute: "objectClass",
			},
			wantSQL:     "e.object_class IS NOT NULL",
			wantArgsLen: 0,
		},
		{
			name: "attribute present",
			filter: &Filter{
				Type:      FilterTypePresent,
				Attribute: "mail",
			},
			wantSQL:     "EXISTS",
			wantArgsLen: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sql, args, err := compiler.CompileToSQL(tt.filter)

			if err != nil {
				t.Errorf("CompileToSQL() error = %v", err)
				return
			}

			if !strings.Contains(sql, tt.wantSQL) {
				t.Errorf("CompileToSQL() SQL = %v, want to contain %v", sql, tt.wantSQL)
			}

			if len(args) != tt.wantArgsLen {
				t.Errorf("CompileToSQL() args length = %v, want %v", len(args), tt.wantArgsLen)
			}
		})
	}
}

func TestCompileSubstring(t *testing.T) {
	compiler := NewFilterCompiler()

	tests := []struct {
		name        string
		filter      *Filter
		wantPattern string
		wantErr     bool
	}{
		{
			name: "initial only",
			filter: &Filter{
				Type:      FilterTypeSubstrings,
				Attribute: "cn",
				Value:     "John*",
			},
			wantPattern: "John%",
		},
		{
			name: "final only",
			filter: &Filter{
				Type:      FilterTypeSubstrings,
				Attribute: "cn",
				Value:     "*Doe",
			},
			wantPattern: "%Doe",
		},
		{
			name: "initial and final",
			filter: &Filter{
				Type:      FilterTypeSubstrings,
				Attribute: "cn",
				Value:     "John*Doe",
			},
			wantPattern: "John%Doe",
		},
		{
			name: "multiple wildcards",
			filter: &Filter{
				Type:      FilterTypeSubstrings,
				Attribute: "cn",
				Value:     "J*oh*oe",
			},
			wantPattern: "J%oh%oe",
		},
		{
			name: "objectClass substring should fail",
			filter: &Filter{
				Type:      FilterTypeSubstrings,
				Attribute: "objectClass",
				Value:     "inet*",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sql, args, err := compiler.CompileToSQL(tt.filter)

			if (err != nil) != tt.wantErr {
				t.Errorf("CompileToSQL() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				return
			}

			if !strings.Contains(sql, "LIKE") {
				t.Errorf("CompileToSQL() SQL should contain LIKE, got: %v", sql)
			}

			if len(args) != 2 {
				t.Errorf("CompileToSQL() args length = %v, want 2", len(args))
				return
			}

			pattern, ok := args[1].(string)
			if !ok {
				t.Errorf("CompileToSQL() second arg should be string pattern")
				return
			}

			if pattern != tt.wantPattern {
				t.Errorf("CompileToSQL() pattern = %v, want %v", pattern, tt.wantPattern)
			}
		})
	}
}

func TestCompileAnd(t *testing.T) {
	compiler := NewFilterCompiler()

	tests := []struct {
		name         string
		filter       *Filter
		wantContains []string
		wantArgsLen  int
	}{
		{
			name: "simple AND with two conditions",
			filter: &Filter{
				Type: FilterTypeAnd,
				Filters: []*Filter{
					{
						Type:      FilterTypeEquality,
						Attribute: "uid",
						Value:     "jdoe",
					},
					{
						Type:      FilterTypeEquality,
						Attribute: "objectClass",
						Value:     "inetOrgPerson",
					},
				},
			},
			wantContains: []string{"AND", "EXISTS"},
			wantArgsLen:  3, // uid, jdoe, inetOrgPerson
		},
		{
			name: "AND with three conditions",
			filter: &Filter{
				Type: FilterTypeAnd,
				Filters: []*Filter{
					{
						Type:      FilterTypeEquality,
						Attribute: "uid",
						Value:     "jdoe",
					},
					{
						Type:      FilterTypePresent,
						Attribute: "mail",
					},
					{
						Type:      FilterTypeEquality,
						Attribute: "cn",
						Value:     "John Doe",
					},
				},
			},
			wantContains: []string{"AND"},
			wantArgsLen:  5, // uid, jdoe, mail, cn, John Doe
		},
		{
			name: "empty AND",
			filter: &Filter{
				Type:    FilterTypeAnd,
				Filters: []*Filter{},
			},
			wantContains: []string{"1=1"},
			wantArgsLen:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sql, args, err := compiler.CompileToSQL(tt.filter)

			if err != nil {
				t.Errorf("CompileToSQL() error = %v", err)
				return
			}

			for _, want := range tt.wantContains {
				if !strings.Contains(sql, want) {
					t.Errorf("CompileToSQL() SQL = %v, want to contain %v", sql, want)
				}
			}

			if len(args) != tt.wantArgsLen {
				t.Errorf("CompileToSQL() args length = %v, want %v", len(args), tt.wantArgsLen)
			}
		})
	}
}

func TestCompileOr(t *testing.T) {
	compiler := NewFilterCompiler()

	tests := []struct {
		name         string
		filter       *Filter
		wantContains []string
		wantArgsLen  int
	}{
		{
			name: "simple OR with two conditions",
			filter: &Filter{
				Type: FilterTypeOr,
				Filters: []*Filter{
					{
						Type:      FilterTypeEquality,
						Attribute: "uid",
						Value:     "jdoe",
					},
					{
						Type:      FilterTypeEquality,
						Attribute: "uid",
						Value:     "jane",
					},
				},
			},
			wantContains: []string{"OR"},
			wantArgsLen:  4, // uid, jdoe, uid, jane
		},
		{
			name: "empty OR",
			filter: &Filter{
				Type:    FilterTypeOr,
				Filters: []*Filter{},
			},
			wantContains: []string{"1=0"},
			wantArgsLen:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sql, args, err := compiler.CompileToSQL(tt.filter)

			if err != nil {
				t.Errorf("CompileToSQL() error = %v", err)
				return
			}

			for _, want := range tt.wantContains {
				if !strings.Contains(sql, want) {
					t.Errorf("CompileToSQL() SQL = %v, want to contain %v", sql, want)
				}
			}

			if len(args) != tt.wantArgsLen {
				t.Errorf("CompileToSQL() args length = %v, want %v", len(args), tt.wantArgsLen)
			}
		})
	}
}

func TestCompileNot(t *testing.T) {
	compiler := NewFilterCompiler()

	tests := []struct {
		name         string
		filter       *Filter
		wantContains []string
		wantErr      bool
	}{
		{
			name: "NOT with single condition",
			filter: &Filter{
				Type: FilterTypeNot,
				Filters: []*Filter{
					{
						Type:      FilterTypeEquality,
						Attribute: "uid",
						Value:     "admin",
					},
				},
			},
			wantContains: []string{"NOT"},
		},
		{
			name: "NOT without sub-filter should fail",
			filter: &Filter{
				Type:    FilterTypeNot,
				Filters: []*Filter{},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sql, _, err := compiler.CompileToSQL(tt.filter)

			if (err != nil) != tt.wantErr {
				t.Errorf("CompileToSQL() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				return
			}

			for _, want := range tt.wantContains {
				if !strings.Contains(sql, want) {
					t.Errorf("CompileToSQL() SQL = %v, want to contain %v", sql, want)
				}
			}
		})
	}
}

func TestCanCompileToSQL(t *testing.T) {
	compiler := NewFilterCompiler()

	tests := []struct {
		name   string
		filter *Filter
		want   bool
	}{
		{
			name: "equality filter",
			filter: &Filter{
				Type:      FilterTypeEquality,
				Attribute: "uid",
				Value:     "jdoe",
			},
			want: true,
		},
		{
			name: "present filter",
			filter: &Filter{
				Type:      FilterTypePresent,
				Attribute: "mail",
			},
			want: true,
		},
		{
			name: "substring filter",
			filter: &Filter{
				Type:      FilterTypeSubstrings,
				Attribute: "cn",
				Value:     "John*",
			},
			want: true,
		},
		{
			name: "AND filter with compilable sub-filters",
			filter: &Filter{
				Type: FilterTypeAnd,
				Filters: []*Filter{
					{Type: FilterTypeEquality, Attribute: "uid", Value: "jdoe"},
					{Type: FilterTypePresent, Attribute: "mail"},
				},
			},
			want: true,
		},
		{
			name: "greater or equal filter (not supported)",
			filter: &Filter{
				Type:      FilterTypeGreaterOrEqual,
				Attribute: "age",
				Value:     "18",
			},
			want: false,
		},
		{
			name: "less or equal filter (not supported)",
			filter: &Filter{
				Type:      FilterTypeLessOrEqual,
				Attribute: "age",
				Value:     "65",
			},
			want: false,
		},
		{
			name: "AND with unsupported sub-filter",
			filter: &Filter{
				Type: FilterTypeAnd,
				Filters: []*Filter{
					{Type: FilterTypeEquality, Attribute: "uid", Value: "jdoe"},
					{Type: FilterTypeGreaterOrEqual, Attribute: "age", Value: "18"},
				},
			},
			want: false,
		},
		{
			name: "NOT with compilable sub-filter",
			filter: &Filter{
				Type: FilterTypeNot,
				Filters: []*Filter{
					{Type: FilterTypeEquality, Attribute: "uid", Value: "admin"},
				},
			},
			want: true,
		},
		{
			name:   "nil filter",
			filter: nil,
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := compiler.CanCompileToSQL(tt.filter)
			if got != tt.want {
				t.Errorf("CanCompileToSQL() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestComplexFilters(t *testing.T) {
	compiler := NewFilterCompiler()

	tests := []struct {
		name         string
		filter       *Filter
		wantContains []string
		wantMinArgs  int
	}{
		{
			name: "nested AND/OR",
			filter: &Filter{
				Type: FilterTypeAnd,
				Filters: []*Filter{
					{
						Type:      FilterTypeEquality,
						Attribute: "objectClass",
						Value:     "inetOrgPerson",
					},
					{
						Type: FilterTypeOr,
						Filters: []*Filter{
							{Type: FilterTypeEquality, Attribute: "uid", Value: "jdoe"},
							{Type: FilterTypeEquality, Attribute: "uid", Value: "jane"},
						},
					},
				},
			},
			wantContains: []string{"AND", "OR"},
			wantMinArgs:  5,
		},
		{
			name: "complex filter with substring",
			filter: &Filter{
				Type: FilterTypeAnd,
				Filters: []*Filter{
					{Type: FilterTypeEquality, Attribute: "objectClass", Value: "inetOrgPerson"},
					{Type: FilterTypeSubstrings, Attribute: "cn", Value: "John*"},
					{Type: FilterTypePresent, Attribute: "mail"},
				},
			},
			wantContains: []string{"AND", "LIKE", "EXISTS"},
			wantMinArgs:  4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sql, args, err := compiler.CompileToSQL(tt.filter)

			if err != nil {
				t.Errorf("CompileToSQL() error = %v", err)
				return
			}

			for _, want := range tt.wantContains {
				if !strings.Contains(sql, want) {
					t.Errorf("CompileToSQL() SQL = %v, want to contain %v", sql, want)
				}
			}

			if len(args) < tt.wantMinArgs {
				t.Errorf("CompileToSQL() args length = %v, want at least %v", len(args), tt.wantMinArgs)
			}
		})
	}
}
