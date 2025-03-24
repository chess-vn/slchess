package stofinet

import "net/http"

type Master struct {
	server *http.Server
}

func NewMaster() *Master {
	router := http.NewServeMux()

	return &Master{
		server: &http.Server{
			Addr:    ":7203",
			Handler: router,
		},
	}
}

func (m *Master) Start() error {
	return m.server.ListenAndServe()
}
