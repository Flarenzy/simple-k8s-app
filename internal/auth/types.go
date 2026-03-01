package auth

type Principal struct {
	Issuer   string
	Subject  string
	Audience any
	Claims   map[string]any
}
