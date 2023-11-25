package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/quickjs-go/quickjs-go"
)

func fetchFunc(qjs *quickjs.Context, this quickjs.Value, args []quickjs.Value) quickjs.Value {
	promise := getPromise(qjs)
	defer promise.Free()

	req, err := prepareReq(qjs, args)
	if err != nil {
		return promise.Reject(err)
	}

	redirected := false
	client := &http.Client{
		Transport: http.DefaultTransport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			switch req.Header.Get("Redirect") {
			case "error":
				return errors.New("redirects are not allowed")
			default:
				if len(via) >= 10 {
					return errors.New("stopped after 10 redirects")
				}
			}

			redirected = true
			return nil
		},
		Timeout: 30 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("a")
		return promise.Reject(err)
	}
	resp.Header.Set("Redirected", fmt.Sprintf("%v", redirected))

	respObj, err := prepareResp(qjs, resp)
	if respObj.IsError() {
		fmt.Println("b")
		return promise.Reject(qjs.Exception())
	}
	if err != nil {
		fmt.Println("c")
		return promise.Reject(err)
	}
	defer respObj.Free()

	return promise.Resolve(respObj)
}

func prepareReq(qjs *quickjs.Context, args []quickjs.Value) (*http.Request, error) {
	if len(args) <= 0 {
		return nil, errors.New("at lease 1 argument required")
	}
	rawURL := args[0].String()

	url, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("url '%s' is not valid", rawURL)
	}

	req, _ := http.NewRequest("GET", url.String(), nil)
	if len(args) > 1 {
		if !args[1].IsObject() {
			return nil, errors.New("2nd argument must be an object")
		}
		req.Body = io.NopCloser(bytes.NewBufferString(args[1].Get("body").String()))
		headers := args[1].Get("headers")
		if keys, err := headers.PropertyNames(); err == nil {
			for _, key := range keys {
				req.Header.Set(key.String(), headers.GetByAtom(key.Atom).String())
			}
		}
		req.Method = args[1].Get("method").String()
	}

	return req, nil
}

func prepareResp(qjs *quickjs.Context, resp *http.Response) (quickjs.Value, error) {
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return qjs.Null(), err
	}

	// jsResp.Status = int32(resp.StatusCode)
	// jsResp.StatusText = resp.Status
	// jsResp.OK = resp.StatusCode >= 200 && resp.StatusCode < 300
	// jsResp.URL = resp.Request.URL.String()

	jsResp := qjs.Object()
	jsResp.Set("status", qjs.Int32(int32(resp.StatusCode)))
	jsResp.Set("statusText", qjs.String(resp.Status))
	jsResp.Set("ok", qjs.Bool(resp.StatusCode >= 200 && resp.StatusCode < 300))
	jsResp.Set("url", qjs.String(resp.Request.URL.String()))
	headers := qjs.Object()
	for k, v := range resp.Header {
		headers.Set(k, qjs.String(strings.Join(v, ",")))
	}
	jsResp.Set("headers", headers)
	jsResp.Set("text", qjs.Function(func(qjs *quickjs.Context, this quickjs.Value, args []quickjs.Value) quickjs.Value {
		promise := getPromise(qjs)
		defer promise.Free()
		return promise.Resolve(qjs.String(string(respBody)))
	}))

	jsResp.Set("json", qjs.Function(func(qjs *quickjs.Context, this quickjs.Value, args []quickjs.Value) quickjs.Value {
		promise := getPromise(qjs)
		defer promise.Free()

		value := make(map[string]any)
		err := json.Unmarshal(respBody, &value)
		if err != nil {
			return promise.Reject(fmt.Errorf("failed to decode json: %w", err))
		}

		return promise.Resolve(asQjsValue(qjs, value))
	}))

	return jsResp, nil
}
