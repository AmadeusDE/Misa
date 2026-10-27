package webui

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
)

//go:embed assets
var assets embed.FS

func staticFS() fs.FS {
	sub, err := fs.Sub(assets, "assets/static")
	if err != nil {
		panic(err)
	}
	return sub
}

var layout = template.Must(template.ParseFS(assets, "assets/layout.html"))

func parsePage(name string) *template.Template {
	return template.Must(template.ParseFS(assets, "assets/"+name))
}

var (
	loginTmpl  = parsePage("login.html")
	enrollTmpl = parsePage("enroll.html")
)

type pageData struct {
	Title     string
	Site      string
	Nav       []NavItem
	CSRF      string
	LoggedIn  bool
	DeviceTag string
	Notices   []Notice
	Data      any
}

func serverFrom(r *http.Request) *Server {
	s, _ := r.Context().Value(serverKey).(*Server)
	return s
}

// Page renders tmpl inside the shared layout. tmpl must define "content"
// (any other templates it defines are available to it); dot inside "content"
// is data. Call it from handlers mounted through Mount or MountPublic.
func Page(w http.ResponseWriter, r *http.Request, title string, tmpl *template.Template, data any) {
	s := serverFrom(r)
	if s == nil {
		http.Error(w, "webui: Page called outside a mounted handler", http.StatusInternalServerError)
		return
	}
	s.render(w, r, title, tmpl, data)
}

func (s *Server) render(w http.ResponseWriter, r *http.Request, title string, tmpl *template.Template, data any) {
	t, err := layout.Clone()
	if err == nil {
		err = adopt(t, tmpl)
	}
	pd := pageData{Title: title, Site: s.opts.Title, Data: data}
	if si, ok := SessionFrom(r); ok {
		pd.LoggedIn, pd.CSRF, pd.DeviceTag = true, si.CSRF, si.DeviceName
		pd.Nav = s.navItems()
		pd.Notices = s.noticesFor(si.DeviceID)
	}
	var buf bytes.Buffer
	if err == nil {
		err = t.ExecuteTemplate(&buf, "layout", pd)
	}
	if err != nil {
		s.opts.Logf("webui: render %q: %v", title, err)
		http.Error(w, "render error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(buf.Bytes())
}

func adopt(dst, src *template.Template) error {
	if src == nil {
		return fmt.Errorf("nil template")
	}
	for _, t := range src.Templates() {
		if t.Tree == nil {
			continue
		}
		if _, err := dst.AddParseTree(t.Name(), t.Tree); err != nil {
			return err
		}
	}
	if src.Lookup("content") == nil {
		if src.Tree == nil {
			return fmt.Errorf("template %q defines no content", src.Name())
		}
		_, err := dst.AddParseTree("content", src.Tree)
		return err
	}
	return nil
}

func (s *Server) noticesFor(deviceID string) []Notice {
	var out []Notice
	_ = s.store.View(func(st *State) {
	next:
		for _, n := range st.Notices {
			for _, d := range n.Dismissed {
				if d == deviceID {
					continue next
				}
			}
			out = append(out, n)
		}
	})
	return out
}

func (s *Server) pageLogin(w http.ResponseWriter, r *http.Request) {
	s.render(w, r, "Sign in", loginTmpl, map[string]string{"Next": safeNext(r.URL.Query().Get("next"))})
}

func (s *Server) pageEnroll(w http.ResponseWriter, r *http.Request) {
	s.render(w, r, "Enroll this device", enrollTmpl, nil)
}
