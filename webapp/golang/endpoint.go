package main

type Endpoint struct {
	Service   string
	Meth      string
	TokenType string
	TokenKey  string
	Uri       string
}

var Services = make(map[string]*Endpoint)

func init() {
	ss := []*Endpoint{
		&Endpoint{"ken", "GET", "", "", "http://api.five-final.isucon.net:8080/%s"},
		&Endpoint{"ken2", "GET", "", "", "http://api.five-final.isucon.net:8080/"},
		&Endpoint{"surname", "GET", "", "", "http://api.five-final.isucon.net:8081/surname"},
		&Endpoint{"givenname", "GET", "", "", "http://api.five-final.isucon.net:8081/givenname"},
		&Endpoint{"tenki", "GET", "param", "zipcode", "http://api.five-final.isucon.net:8988/"},
		&Endpoint{"perfectsec", "GET", "header", "X-PERFECT-SECURITY-TOKEN", "https://api.five-final.isucon.net:8443/tokens"},
		&Endpoint{"perfectsec_attacked", "GET", "header", "X-PERFECT-SECURITY-TOKEN", "https://api.five-final.isucon.net:8443/attacked_list"},
	}
	for _, e := range ss {
		Services[e.Service] = e
	}
}
