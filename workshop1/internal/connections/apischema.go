package connections

import (
	"sync"
)

type ApiSchema struct {
	method         string
	uri            string
	requestSchema  string
	responseSchema string
	containsPII    bool

	mutex sync.RWMutex
}

func NewApiSchema(method string, uri string, requestSchema string, responseSchema string, containsPII bool) *ApiSchema {
	return &ApiSchema{
		method:         method,
		uri:            uri,
		requestSchema:  requestSchema,
		responseSchema: responseSchema,
		containsPII:    containsPII,
		mutex:          sync.RWMutex{},
	}
}
