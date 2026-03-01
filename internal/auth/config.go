package auth

type Config struct {
	Enabled  bool
	Issuer   string
	Audience string
	JWKSURL  string
}
