package domain

type CreateSubnetInput struct {
	CIDR        string
	Description string
}

type CreateIPInput struct {
	IP       string
	Hostname string
}

type UpdateIPInput struct {
	Hostname string
}
