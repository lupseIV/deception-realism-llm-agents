package facts

// Fact keys shared by the assembler and the layers. Keep every key here so
// that layers never spell one by hand.
const (
	RunID   = "run.id"
	RunSeed = "run.seed" // decimal

	PersonaID          = "persona.id"
	PersonaUser        = "persona.user"
	PersonaFullName    = "persona.full-name"
	PersonaHostname    = "persona.hostname"
	PersonaCompany     = "persona.company"
	PersonaCompanySlug = "persona.company-slug"
	PersonaRole        = "persona.role"
	PersonaRegion      = "persona.region"
	PersonaHome        = "persona.home" // /home/<user>

	tokenPathPrefix = "persona.token-path."
)

// TokenPathKey returns the key holding the persona's placement path for a
// token type, e.g. "persona.token-path.aws-credentials". The value keeps the
// leading ~ as written in the persona file.
func TokenPathKey(tokenType string) string { return tokenPathPrefix + tokenType }
