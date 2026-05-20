package app

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"
	"text/template"
)

type TemplateRegistry struct {
	templates map[string]Template
}

func NewTemplateRegistry() *TemplateRegistry {
	r := &TemplateRegistry{templates: map[string]Template{}}
	for _, t := range builtins() {
		r.templates[t.ID] = t
	}
	return r
}

func (r *TemplateRegistry) List() []Template {
	out := make([]Template, 0, len(r.templates))
	for _, t := range r.templates {
		out = append(out, t)
	}
	return out
}

func (r *TemplateRegistry) Get(id string) (Template, bool) {
	t, ok := r.templates[id]
	return t, ok
}

func (r *TemplateRegistry) Add(t Template) {
	r.templates[t.ID] = t
}

func (r *TemplateRegistry) Render(id string, params map[string]string) (string, []string, error) {
	t, ok := r.Get(id)
	if !ok {
		return "", nil, fmt.Errorf("template %q not found", id)
	}
	warnings := []string{}
	for _, p := range t.Params {
		if _, ok := params[p.Name]; !ok && p.Default != "" {
			params[p.Name] = p.Default
		}
		if p.Required && strings.TrimSpace(params[p.Name]) == "" {
			return "", nil, fmt.Errorf("missing required parameter %s", p.Name)
		}
	}
	if t.RiskLevel == "high" {
		warnings = append(warnings, "This is a high-risk template. Review every generated resource before applying.")
	}
	out := []string{}
	for _, res := range t.Resources {
		rendered, err := renderText(res.Template, params)
		if err != nil {
			return "", nil, err
		}
		out = append(out, strings.TrimSpace(rendered))
	}
	return strings.Join(out, "\n---\n") + "\n", warnings, nil
}

func renderText(src string, params map[string]string) (string, error) {
	tpl, err := template.New("resource").Funcs(template.FuncMap{"dns": dns1123}).Parse(src)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := tpl.Execute(&buf, params); err != nil {
		return "", err
	}
	return buf.String(), nil
}

var nonDNS = regexp.MustCompile(`[^a-z0-9-]+`)

func dns1123(in string) string {
	s := strings.ToLower(strings.TrimSpace(in))
	s = nonDNS.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if s == "" {
		return "default"
	}
	return s
}
