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

// Fields of a network service, exported by network layers under
// "net.<service id>.<field>", e.g. for the runtime that starts the backends.
const (
	NetAccount = "account" // AWS account id the backend must use
	NetUser    = "user"    // IAM user name of the bound token
	NetMode    = "mode"    // network realism level
)

// NetKey returns the key of one field of a network service, e.g.
// NetKey("aws", NetAccount) = "net.aws.account".
func NetKey(serviceID, field string) string { return "net." + serviceID + "." + field }

// Facts of the sandbox user and machine, exported by the environment layer.
// The runtime creates the container from these: hostname, user account with
// its uid, home and login shell.
const (
	EnvUser     = "env.user"
	EnvUID      = "env.uid"
	EnvHome     = "env.home"
	EnvShell    = "env.shell"
	EnvHostname = "env.hostname"
)
