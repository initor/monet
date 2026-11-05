package charter

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// Hash returns the lowercase hex SHA-256 of data.
func Hash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// Render returns charter.md, the page that the owner confirms. Its last line
// holds hash, so seal can tell when charter.json changed after the render.
func Render(c *Charter, hash string) string {
	var b strings.Builder
	section := func(title, body string) { b.WriteString("## " + title + "\n\n" + body + "\n\n") }
	b.WriteString("# Charter\n\n")
	section("Goal", c.Goal)
	section("Context", c.Context)
	section("Deliverable", c.Deliverable)
	section("Phases", bullets(c.Phases))
	section("Quality bar", c.QualityBar)
	var invs, cards, grants []string
	for _, inv := range c.Invariants {
		invs = append(invs, inv.ID+": "+inv.Text)
	}
	section("Invariants", bullets(invs))
	for _, p := range c.Personas {
		cards = append(cards, p.ID+" ("+p.Weight+"): "+p.Description+
			"\n  - sources: "+list(p.Sources)+"\n  - phases: "+list(p.Phases))
	}
	section("Personas", bullets(cards))
	section("Page", c.Page)
	for _, g := range c.Grants {
		grants = append(grants, g.Step+" (words "+g.Words+")")
	}
	section("Grants", bullets(grants))
	b.WriteString("<!-- charter sha256=" + hash + " -->\n")
	return b.String()
}

// bullets returns items as a Markdown list, or "- none" when there are none.
func bullets(items []string) string {
	if len(items) == 0 {
		return "- none"
	}
	return "- " + strings.Join(items, "\n- ")
}

// list returns items joined by commas, or "none" when there are none.
func list(items []string) string {
	if len(items) == 0 {
		return "none"
	}
	return strings.Join(items, ", ")
}
