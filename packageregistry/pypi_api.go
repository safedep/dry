package packageregistry

type pypiPackage struct {
	Info struct {
		Author      string `json:"author"`
		AuthorEmail string `json:"author_email"`
	} `json:"info"`
}
