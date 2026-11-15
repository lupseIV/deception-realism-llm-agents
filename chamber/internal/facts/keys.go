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

// Fields of a planted token, exported by honeytoken layers under
// "token.<token id>.<field>" for the layers bound to that token.
const (
	TokenType            = "type"
	TokenSource          = "source" // "generator", "pool:<origin>", ...
	TokenPath            = "path"   // absolute path of the planted file
	TokenAccessKeyID     = "access-key-id"
	TokenSecretAccessKey = "secret-access-key"
	TokenProfile         = "profile"
	TokenBucket          = "bucket" // high realism only
	TokenRegion          = "region"
)

// TokenKey returns the key of one field of a planted token, e.g.
// TokenKey("creds", TokenBucket) = "token.creds.bucket".
func TokenKey(tokenID, field string) string { return "token." + tokenID + "." + field }

// TokenPathKey returns the key holding the persona's placement path for a
// token type, e.g. "persona.token-path.aws-credentials". The value keeps the
// leading ~ as written in the persona file.
func TokenPathKey(tokenType string) string { return tokenPathPrefix + tokenType }
