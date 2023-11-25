package main

import (
	"github.com/quickjs-go/quickjs-go"
)

type jsThreadInterface struct{}

func qjsStringArray(qjs *quickjs.Context, src []string) quickjs.Value {
	arr := qjs.Array()
	for j, item := range src {
		arr.SetByUint32(uint32(j), qjs.String(item))
	}
	return arr
}

func qjsIntArray(qjs *quickjs.Context, src []int) quickjs.Value {
	arr := qjs.Array()
	for j, item := range src {
		arr.SetByUint32(uint32(j), qjs.Int32(int32(item)))
	}
	return arr
}

func asQjsValue(qjs *quickjs.Context, anything any) quickjs.Value {
	var jsValue quickjs.Value

	switch value := anything.(type) {
	case string:
		jsValue = qjs.String(value)
	case bool:
		jsValue = qjs.Bool(value)
	case float64:
		jsValue = qjs.Float64(value)
	case int:
		jsValue = qjs.Int32(int32(value))
	case nil:
		jsValue = qjs.Null()
	case map[string]any:
		jsValue = qjs.Object()
		for k, v := range value {
			jsValue.Set(k, asQjsValue(qjs, v))
		}
	case []any:
		jsValue = qjs.Array()
		for i, v := range value {
			jsValue.SetByUint32(uint32(i), asQjsValue(qjs, v))
		}
	}
	return jsValue
}

type JSPromise struct {
	Promise quickjs.Value

	resolve quickjs.Value
	reject  quickjs.Value

	Resolve func(quickjs.Value) quickjs.Value
	Reject  func(error) quickjs.Value
}

func (jsp JSPromise) Free() {
	jsp.Promise.Free()
	jsp.resolve.Free()
	jsp.reject.Free()
}

func getPromise(qjs *quickjs.Context) JSPromise {
	qjs.Eval(`
globalThis.promise = new Promise((rsv, rjc) => {
  globalThis.resolve = rsv
  globalThis.reject = rjc
})
`, quickjs.EVAL_GLOBAL)

	p := qjs.DupValue(qjs.Globals().Get("promise"))
	resolve := qjs.DupValue(qjs.Globals().Get("resolve"))
	reject := qjs.DupValue(qjs.Globals().Get("reject"))

	return JSPromise{
		Promise: p,
		resolve: resolve,
		reject:  reject,

		Resolve: func(val quickjs.Value) quickjs.Value {
			v, _ := qjs.Call(qjs.Null(), resolve, []quickjs.Value{val})
			v.Free()
			return p
		},
		Reject: func(err error) quickjs.Value {
			v, _ := qjs.Call(qjs.Null(), reject, []quickjs.Value{qjs.Error(err)})
			v.Free()
			return p
		},
	}
}
