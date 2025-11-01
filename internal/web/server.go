package web

import (
	"context"
	"embed"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/smarzola/ldaplite/internal/store"
	"github.com/smarzola/ldaplite/internal/web/handlers"
	"github.com/smarzola/ldaplite/internal/web/middleware"
	"github.com/smarzola/ldaplite/pkg/config"
)

//go:embed templates/*.html
var templatesFS embed.FS

//go:embed static/output.css
var staticCSS embed.FS

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

	// Serve static CSS (no auth required)
	s.mux.Handle("/static/", http.FileServer(http.FS(staticCSS)))

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
		w.Header().Set("WWW-Authenticate", `Basic realm="LDAPLite Admin"`)
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte("Logged out successfully. Close this browser tab."))
	})

	// User routes
	s.mux.Handle("/users", auth.RequireAuth(http.HandlerFunc(userHandler.List)))
	s.mux.Handle("/users/new", auth.RequireAuth(http.HandlerFunc(userHandler.New)))
	s.mux.Handle("/users/edit", auth.RequireAuth(http.HandlerFunc(userHandler.Edit)))
	s.mux.Handle("/users/delete", auth.RequireAuth(http.HandlerFunc(userHandler.Delete)))

	// Group routes
	s.mux.Handle("/groups", auth.RequireAuth(http.HandlerFunc(groupHandler.List)))
	s.mux.Handle("/groups/new", auth.RequireAuth(http.HandlerFunc(groupHandler.New)))
	s.mux.Handle("/groups/edit", auth.RequireAuth(http.HandlerFunc(groupHandler.Edit)))
	s.mux.Handle("/groups/delete", auth.RequireAuth(http.HandlerFunc(groupHandler.Delete)))

	// OU routes
	s.mux.Handle("/ous", auth.RequireAuth(http.HandlerFunc(ouHandler.List)))
	s.mux.Handle("/ous/new", auth.RequireAuth(http.HandlerFunc(ouHandler.New)))
	s.mux.Handle("/ous/edit", auth.RequireAuth(http.HandlerFunc(ouHandler.Edit)))
	s.mux.Handle("/ous/delete", auth.RequireAuth(http.HandlerFunc(ouHandler.Delete)))
}

// Start starts the web server
func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%d", s.cfg.WebUI.BindAddress, s.cfg.WebUI.Port)
	slog.Info("Starting web UI server", "address", addr)

	s.httpServer = &http.Server{
		Addr:    addr,
		Handler: s.mux,
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
