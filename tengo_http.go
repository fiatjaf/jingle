package main

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/d5/tengo/v2"
)

var tengoHttp = map[string]tengo.Object{
	"get": &tengo.UserFunction{
		Name: "http.get",
		Value: tengo.CallableFunc(func(args ...tengo.Object) (ret tengo.Object, err error) {
			if len(args) < 1 {
				return nil, fmt.Errorf("http.get() needs an argument")
			}

			url, ok := args[0].(*tengo.String)
			if !ok {
				return nil, fmt.Errorf("http.get() argument must be a string")
			}

			resp, err := http.Get(url.Value)
			if err != nil {
				return nil, fmt.Errorf("http.get() failed to call '%s': %w", url.Value, err)
			}
			defer resp.Body.Close()

			if resp.StatusCode >= 300 {
				return nil, fmt.Errorf("http.get() got a status code %d from '%s'", resp.StatusCode, url.Value)
			}

			b, err := io.ReadAll(resp.Body)
			if err != nil {
				return nil, fmt.Errorf("http.get() failed to read response from '%s': %w", url.Value, err)
			}

			return &tengo.String{Value: strings.TrimSpace(string(b))}, nil
		}),
	},
}
