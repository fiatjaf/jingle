package main

import (
	"context"
	"sync"

	"github.com/fiatjaf/khatru"
	"github.com/fiatjaf/quickjs-go"
	"github.com/puzpuzpuz/xsync/v2"
)

var sessionStorage = xsync.NewTypedMapOf[*khatru.WebSocket, store](pointerHasher)

type store struct {
	data  map[string]string
	mutex sync.Mutex
}

var globalStore = store{data: make(map[string]string)}

func onDisconnect(ctx context.Context) {
	sessionStorage.Delete(khatru.GetConnection(ctx))
}

func makeRelayObject(ctx context.Context, qjs *quickjs.Context) quickjs.Value {
	relayObject := qjs.Object()

	queryFunc := qjs.Function(func(qjs *quickjs.Context, this quickjs.Value, args []quickjs.Value) quickjs.Value {
		filterjs := args[0] // this is expected to be a nostr filter object
		filter := filterFromJs(qjs, filterjs)
		events, err := wrapper.QuerySync(ctx, filter)
		if err != nil {
			qjs.ThrowError(err)
		}
		results := qjs.Array()
		for _, event := range events {
			results.Push(eventToJs(qjs, event))
		}
		return results.ToValue()
	})
	relayObject.Set("query", queryFunc)

	setFunc := qjs.Function(func(qjs *quickjs.Context, this quickjs.Value, args []quickjs.Value) quickjs.Value {
		k := args[0].String()
		v := args[1].JSONStringify()
		globalStore.mutex.Lock()
		globalStore.data[k] = v
		globalStore.mutex.Unlock()
		return qjs.Undefined()
	})
	getFunc := qjs.Function(func(qjs *quickjs.Context, this quickjs.Value, args []quickjs.Value) quickjs.Value {
		k := args[0].String()
		globalStore.mutex.Lock()
		v := qjs.ParseJSON(globalStore.data[k])
		globalStore.mutex.Unlock()
		return v
	})
	delFunc := qjs.Function(func(qjs *quickjs.Context, this quickjs.Value, args []quickjs.Value) quickjs.Value {
		k := args[0].String()
		globalStore.mutex.Lock()
		delete(globalStore.data, k)
		globalStore.mutex.Unlock()
		return qjs.Undefined()
	})

	store := qjs.Object()
	store.Set("set", setFunc)
	store.Set("get", getFunc)
	store.Set("del", delFunc)
	relayObject.Set("store", store)

	return relayObject
}

func makeConnectionObject(ctx context.Context, qjs *quickjs.Context) quickjs.Value {
	connObject := qjs.Object()
	connObject.Set("ip", qjs.String(khatru.GetIP(ctx)))
	if pubkey := khatru.GetAuthed(ctx); pubkey != "" {
		connObject.Set("pubkey", qjs.String(pubkey))
	}
	connObject.Set("getOpenSubscriptions", qjs.Function(func(qjs *quickjs.Context, this quickjs.Value, args []quickjs.Value) quickjs.Value {
		subs := qjs.Array()
		for _, filter := range khatru.GetOpenSubscriptions(ctx) {
			subs.Push(filterToJs(qjs, filter))
		}
		return subs.ToValue()
	}))

	setFunc := qjs.Function(func(qjs *quickjs.Context, this quickjs.Value, args []quickjs.Value) quickjs.Value {
		k := args[0].String()
		v := args[1].JSONStringify()
		s, _ := sessionStorage.LoadOrCompute(khatru.GetConnection(ctx), func() store {
			return store{data: make(map[string]string)}
		})
		s.mutex.Lock()
		s.data[k] = v
		s.mutex.Unlock()
		return qjs.Undefined()
	})
	getFunc := qjs.Function(func(qjs *quickjs.Context, this quickjs.Value, args []quickjs.Value) quickjs.Value {
		k := args[0].String()
		s, ok := sessionStorage.Load(khatru.GetConnection(ctx))
		if !ok {
			return qjs.Undefined()
		}
		s.mutex.Lock()
		v := qjs.ParseJSON(s.data[k])
		s.mutex.Unlock()
		return v
	})
	delFunc := qjs.Function(func(qjs *quickjs.Context, this quickjs.Value, args []quickjs.Value) quickjs.Value {
		k := args[0].String()
		s, ok := sessionStorage.Load(khatru.GetConnection(ctx))
		if ok {
			s.mutex.Lock()
			delete(s.data, k)
			s.mutex.Unlock()
		}
		return qjs.Undefined()
	})

	store := qjs.Object()
	store.Set("set", setFunc)
	store.Set("get", getFunc)
	store.Set("del", delFunc)
	connObject.Set("store", store)

	return connObject
}
