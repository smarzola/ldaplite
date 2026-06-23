package web

import (
	"context"
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/smarzola/ldaplite/internal/authz"
	"github.com/smarzola/ldaplite/internal/store"
	"github.com/smarzola/ldaplite/internal/web/handlers"
	"github.com/smarzola/ldaplite/internal/web/middleware"
	"github.com/smarzola/ldaplite/pkg/config"
)

//go:embed templates/*.html
var templatesFS embed.FS

//go:embed static
var staticFS embed.FS

// Server represents the web UI HTTP server
type Server struct {
	cfg        *config.Config
	store      store.Store
	templates  *template.Template
	mux        *http.ServeMux
	httpServer *http.Server
}

// NewServer creates a new web UI server
func NewServer(cfg *config.Config, st store.Store) (*Server, error) {
	s := &Server{
		cfg:       cfg,
		store:     st,
		templates: nil, // Templates are parsed per-request to avoid block name conflicts
		mux:       http.NewServeMux(),
	}

	// Setup routes
	s.setupRoutes()

	return s, nil
}

// GetTemplate parses a specific template with base.html to avoid block name conflicts
func (s *Server) GetTemplate(name string) (*template.Template, error) {
	// Define custom template functions
	funcMap := template.FuncMap{
		"upper": strings.ToUpper,
	}

	// Create template with custom functions
	tmpl := template.New("").Funcs(funcMap)
	return tmpl.ParseFS(templatesFS, "templates/base.html", "templates/"+name)
}

// setupRoutes configures all HTTP routes
func (s *Server) setupRoutes() {
	// Create auth middleware
	auth := middleware.NewAuth(s.store, s.cfg)

	// Create handlers
	userHandler := handlers.NewUserHandler(s.store, s.cfg, s.GetTemplate)
	groupHandler := handlers.NewGroupHandler(s.store, s.cfg, s.GetTemplate)
	ouHandler := handlers.NewOUHandler(s.store, s.cfg, s.GetTemplate)
	apiHandler := handlers.NewAPIHandler(s.store, s.cfg)
	readProtected := func(handler http.HandlerFunc) http.Handler {
		return auth.RequireCapability(authz.UIRead, handler)
	}
	adminProtected := func(handler http.HandlerFunc) http.Handler {
		return auth.RequireCapability(authz.UIAdmin, middleware.RequireSameOrigin(handler))
	}
	passwordSelfProtected := func(handler http.HandlerFunc) http.Handler {
		return auth.RequireCapability(authz.PasswordChangeSelf, middleware.RequireSameOrigin(handler))
	}
	passwordResetProtected := func(handler http.HandlerFunc) http.Handler {
		return auth.RequireCapability(authz.PasswordResetAny, middleware.RequireSameOrigin(handler))
	}

	// Serve static CSS (no auth required)
	s.mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(mustSubFS(staticFS, "static")))))

	// Serve the embedded React/shadcn application.
	s.mux.HandleFunc("/app", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/app/", http.StatusMovedPermanently)
	})
	s.mux.Handle("/app/", auth.RequireCapability(authz.UIRead, http.StripPrefix("/app/", spaFileServer(mustSubFS(staticFS, "static/app")))))

	// Root redirect
	s.mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.Redirect(w, r, "/users", http.StatusFound)
			return
		}
		http.NotFound(w, r)
	})

	// Logout handler
	s.mux.HandleFunc("/logout", func(w http.ResponseWriter, r *http.Request) {
		// Send 401 to clear browser's auth cache
		w.Header().Set("WWW-Authenticate", `Basic realm="LDAPLite Web UI"`)
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte("Logged out successfully. Close this browser tab."))
	})

	// API routes
	s.mux.Handle("/api/session", readProtected(apiHandler.Session))
	s.mux.Handle("/api/directory", auth.RequireCapability(authz.DirectoryRead, http.HandlerFunc(apiHandler.Directory)))
	s.mux.Handle("/api/users", adminProtected(apiHandler.Users))
	s.mux.Handle("/api/groups", adminProtected(apiHandler.Groups))
	s.mux.Handle("/api/ous", adminProtected(apiHandler.OUs))
	s.mux.Handle("/api/account/password", passwordSelfProtected(apiHandler.ChangeOwnPassword))
	s.mux.Handle("/api/users/password", passwordResetProtected(apiHandler.ResetPassword))

	// User routes
	s.mux.Handle("/users", readProtected(userHandler.List))
	s.mux.Handle("/users/new", adminProtected(userHandler.New))
	s.mux.Handle("/users/edit", adminProtected(userHandler.Edit))
	s.mux.Handle("/users/delete", adminProtected(userHandler.Delete))

	// Group routes
	s.mux.Handle("/groups", readProtected(groupHandler.List))
	s.mux.Handle("/groups/new", adminProtected(groupHandler.New))
	s.mux.Handle("/groups/edit", adminProtected(groupHandler.Edit))
	s.mux.Handle("/groups/delete", adminProtected(groupHandler.Delete))

	// OU routes
	s.mux.Handle("/ous", readProtected(ouHandler.List))
	s.mux.Handle("/ous/new", adminProtected(ouHandler.New))
	s.mux.Handle("/ous/edit", adminProtected(ouHandler.Edit))
	s.mux.Handle("/ous/delete", adminProtected(ouHandler.Delete))
}

func mustSubFS(fsys fs.FS, dir string) fs.FS {
	sub, err := fs.Sub(fsys, dir)
	if err != nil {
		panic(fmt.Sprintf("embedded filesystem %q unavailable: %v", dir, err))
	}
	return sub
}

func spaFileServer(fsys fs.FS) http.Handler {
	files := http.FileServer(http.FS(fsys))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}

		file, err := fsys.Open(path)
		if err == nil {
			_ = file.Close()
			files.ServeHTTP(w, r)
			return
		}

		fallback := r.Clone(r.Context())
		fallback.URL.Path = "/index.html"
		files.ServeHTTP(w, fallback)
	})
}

// Start starts the web server
func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%d", s.cfg.WebUI.BindAddress, s.cfg.WebUI.Port)
	slog.Info("Starting web UI server", "address", addr)

	s.httpServer = &http.Server{
		Addr:    addr,
		Handler: middleware.AuditHTTP(s.mux),
	}

	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("web server failed: %w", err)
	}

	return nil
}

// Stop gracefully shuts down the web server
func (s *Server) Stop() error {
	if s.httpServer == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := s.httpServer.Shutdown(ctx); err != nil {
		return fmt.Errorf("web server shutdown failed: %w", err)
	}

	return nil
}
