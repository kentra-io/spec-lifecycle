package spec

import "strings"

// Render serializes a RequirementSet back to spec.md bytes: the inverse of
// ParseRequirementSet. Every requirement's Raw block is re-emitted
// verbatim; only the separators BETWEEN blocks are canonical (a single
// blank line between requirement blocks and between Before/Preamble and the
// "## Requirements" header, mirroring OpenSpec buildUpdatedSpec's own
// join-then-collapse: `[before, headerLine, reqBody, after].join('\n')`
// followed by collapsing runs of 3+ newlines to exactly one blank line).
//
// When there is no "## Requirements" section to render at all
// (!HasRequirementsSection, no requirements, and no preamble), Render does
// NOT invent one: it reproduces Before/After only. Inventing structure that
// was never in the source would break the render(parse(b)) fixed-point
// property for a document this package cannot recognize as spec-shaped
// (e.g. one entirely swallowed by an unterminated code fence).
func (rs *RequirementSet) Render() []byte {
	if !rs.HasRequirementsSection && len(rs.Requirements) == 0 && strings.TrimSpace(rs.Preamble) == "" {
		out := trimTrailingWS(rs.Before) + rs.After
		return []byte(blankLinesRe.ReplaceAllString(out, "\n\n"))
	}

	reqBodyParts := make([]string, 0, len(rs.Requirements)+1)
	if strings.TrimSpace(rs.Preamble) != "" {
		// Emit Preamble as stored (right-trimmed only, per
		// ParseRequirementSet) — NOT strings.TrimSpace(rs.Preamble).
		// Stripping its leading whitespace here would be destructive in a
		// way plain trailing-whitespace trimming isn't: leading spaces
		// can be exactly what stopped a line from being recognized as an
		// "## " header on the first parse (h2Re is anchored, `^##\s+`);
		// discarding them would let that line start matching as a real
		// header on the NEXT parse, changing where the Requirements
		// section is judged to end and breaking the render(parse(b))
		// fixed-point property (caught by FuzzParseRenderRoundTrip).
		reqBodyParts = append(reqBodyParts, rs.Preamble)
	}
	for _, r := range rs.Requirements {
		reqBodyParts = append(reqBodyParts, r.Raw)
	}
	reqBody := trimTrailingWS(strings.Join(reqBodyParts, "\n\n"))

	segs := []string{
		trimTrailingWS(rs.Before),
		"## Requirements",
		reqBody,
		rs.After,
	}
	// Mirror the oracle exactly: drop the leading segment only if it's
	// empty (a brand-new capability with no title/Purpose yet); keep every
	// other segment even when empty, since join still needs to reproduce
	// the right number of separating newlines.
	if segs[0] == "" {
		segs = segs[1:]
	}

	out := strings.Join(segs, "\n")
	out = blankLinesRe.ReplaceAllString(out, "\n\n")
	return []byte(out)
}
