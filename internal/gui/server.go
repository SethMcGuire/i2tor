package gui

import (
	"context"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"sync"
	"time"
)

type ViewModel struct {
	Title      string
	Subtitle   string
	LastAction string
	LastError  string
	Fields     map[string]string
}

type Server struct {
	mu       sync.Mutex
	model    ViewModel
	actions  map[string]func(context.Context) (string, error)
	template *template.Template
}

func New(model ViewModel, actions map[string]func(context.Context) (string, error)) (*Server, error) {
	tpl, err := template.New("gui").Parse(pageTemplate)
	if err != nil {
		return nil, err
	}
	return &Server{
		model:    model,
		actions:  actions,
		template: tpl,
	}, nil
}

func (s *Server) Serve(ctx context.Context, addr string) (string, error) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/action", s.handleAction)

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return "", fmt.Errorf("listen on %s: %w", addr, err)
	}

	srv := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()
	go func() {
		_ = srv.Serve(listener)
	}()
	return "http://" + listener.Addr().String(), nil
}

func (s *Server) UpdateModel(model ViewModel) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.model = model
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	s.mu.Lock()
	model := s.model
	s.mu.Unlock()
	if err := s.template.Execute(w, model); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) handleAction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	actionName := r.FormValue("name")
	action, ok := s.actions[actionName]
	if !ok {
		http.Error(w, "unknown action", http.StatusBadRequest)
		return
	}
	msg, err := action(r.Context())
	s.mu.Lock()
	if err != nil {
		s.model.LastError = err.Error()
		s.model.LastAction = ""
	} else {
		s.model.LastAction = msg
		s.model.LastError = ""
	}
	s.mu.Unlock()
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

const pageTemplate = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>{{.Title}}</title>
  <style>
    :root { color-scheme: light; --bg:#f3efe5; --card:#fffdf7; --ink:#18211f; --muted:#5a6863; --accent:#0b6e4f; --danger:#8f1d2c; --line:#d8d2c5; }
    body { margin:0; font-family: Georgia, "Iowan Old Style", serif; background: radial-gradient(circle at top left, #fff7df 0, var(--bg) 45%, #e6ede9 100%); color:var(--ink); }
    main { max-width: 900px; margin: 32px auto; padding: 24px; }
    .card { background: var(--card); border:1px solid var(--line); border-radius: 18px; padding: 24px; box-shadow: 0 12px 40px rgba(24,33,31,.08); }
    h1 { margin:0 0 8px; font-size: 40px; }
    p { color: var(--muted); }
    .grid { display:grid; grid-template-columns: repeat(auto-fit, minmax(210px, 1fr)); gap: 12px; margin: 24px 0; }
    button { width:100%; padding: 14px 16px; border-radius: 14px; border:0; background: var(--accent); color:white; font-size:16px; cursor:pointer; }
    .status { display:grid; grid-template-columns: repeat(auto-fit, minmax(220px, 1fr)); gap: 10px; margin-top: 24px; }
    .status div { background:#fbf9f2; border:1px solid var(--line); border-radius:14px; padding:12px 14px; }
    .label { display:block; font-size:12px; text-transform:uppercase; letter-spacing:.08em; color:var(--muted); margin-bottom:4px; }
    .error { color: var(--danger); font-weight: bold; }
    .ok { color: var(--accent); font-weight: bold; }
  </style>
</head>
<body>
<main>
  <section class="card">
    <h1>{{.Title}}</h1>
    <p>{{.Subtitle}}</p>
    {{if .LastAction}}<p class="ok">{{.LastAction}}</p>{{end}}
    {{if .LastError}}<p class="error">{{.LastError}}</p>{{end}}
    <div class="grid">
      <form method="post" action="/action"><input type="hidden" name="name" value="install"><button type="submit">Install</button></form>
      <form method="post" action="/action"><input type="hidden" name="name" value="update"><button type="submit">Update</button></form>
      <form method="post" action="/action"><input type="hidden" name="name" value="doctor"><button type="submit">Doctor</button></form>
      <form method="post" action="/action"><input type="hidden" name="name" value="run"><button type="submit">Run</button></form>
    </div>
    <div class="status">
      {{range $key, $value := .Fields}}
      <div><span class="label">{{$key}}</span>{{$value}}</div>
      {{end}}
    </div>
  </section>
</main>
</body>
</html>`
