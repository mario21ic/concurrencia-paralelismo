package main

import (
	"net/url"
	"regexp"
	"strings"
)

var reHref = regexp.MustCompile(`(?i)href\s*=\s*["']([^"']+)["']`)

// ExtraerEnlaces devuelve los enlaces absolutos (http/https) de un documento
// HTML, resueltos respecto a `base`, sin fragmento y sin duplicados.
//
// Limitación consciente: usa una expresión regular, suficiente para este
// ejercicio. Para HTML real usa un analizador (golang.org/x/net/html).
func ExtraerEnlaces(base *url.URL, html string) []string {
	vistos := make(map[string]struct{})
	var out []string
	for _, m := range reHref.FindAllStringSubmatch(html, -1) {
		ref := strings.TrimSpace(m[1])
		if ref == "" || strings.HasPrefix(ref, "mailto:") || strings.HasPrefix(ref, "javascript:") {
			continue
		}
		u, err := base.Parse(ref)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
			continue
		}
		u.Fragment = ""
		s := u.String()
		if _, ok := vistos[s]; ok {
			continue
		}
		vistos[s] = struct{}{}
		out = append(out, s)
	}
	return out
}
