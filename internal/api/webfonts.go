package api

import (
	"embed"
	"net/http"
	"path"
	"strings"
)

// The website's three typefaces, served from the binary itself so that no page loads
// anything from another host: Familjen Grotesk for headings, IBM Plex Sans for text and
// JetBrains Mono for code. All SIL Open Font License 1.1; where they came from is in
// webfonts/SHA256SUMS.

//go:embed webfonts/*.woff2 webfonts/SHA256SUMS
var webFonts embed.FS

// fontFaces is the @font-face block at the top of the site's styles. Only the weights
// the pages use are shipped.
const fontFaces = `@font-face{font-family:"Familjen Grotesk";font-style:normal;font-weight:500;font-display:swap;src:url(/fonts/familjen-grotesk-latin-500-normal.woff2) format("woff2")}
@font-face{font-family:"Familjen Grotesk";font-style:normal;font-weight:600;font-display:swap;src:url(/fonts/familjen-grotesk-latin-600-normal.woff2) format("woff2")}
@font-face{font-family:"IBM Plex Sans";font-style:normal;font-weight:400;font-display:swap;src:url(/fonts/ibm-plex-sans-latin-400-normal.woff2) format("woff2")}
@font-face{font-family:"IBM Plex Sans";font-style:normal;font-weight:500;font-display:swap;src:url(/fonts/ibm-plex-sans-latin-500-normal.woff2) format("woff2")}
@font-face{font-family:"JetBrains Mono";font-style:normal;font-weight:400;font-display:swap;src:url(/fonts/jetbrains-mono-latin-400-normal.woff2) format("woff2")}
`

func (s *Server) handleFont(w http.ResponseWriter, r *http.Request) {
	name := path.Base(r.PathValue("file"))
	data, err := webFonts.ReadFile("webfonts/" + name)
	if err != nil || !strings.HasSuffix(name, ".woff2") {
		s.notFound(w, r, "No such font.")
		return
	}
	writeBody(w, r, http.StatusOK, data, "font/woff2", cacheLong, false)
}
