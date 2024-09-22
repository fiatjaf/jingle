package main

import (
	"context"
	"fmt"
	"sync"

	"github.com/d5/tengo/v2"
	"github.com/fiatjaf/khatru"
	"github.com/puzpuzpuz/xsync/v2"
)

var sessionStorage = xsync.NewTypedMapOf[*khatru.WebSocket, store](pointerHasher)

type store struct {
	data  map[string]any
	mutex sync.Mutex
}

var globalStore = store{data: make(map[string]any)}

func onDisconnect(ctx context.Context) {
	sessionStorage.Delete(khatru.GetConnection(ctx))
}

func makeRelayObject(ctx context.Context) tengo.Object {
	return &tengo.Map{
		Value: map[string]tengo.Object{
			"query": &tengo.UserFunction{
				Name: "query",
				Value: tengo.CallableFunc(func(args ...tengo.Object) (tengo.Object, error) {
					if len(args) == 0 {
						return nil, fmt.Errorf("query function requires an argument")
					}
					filter, err := filterFromTengo(args[0])

					ch, err := db.QueryEvents(ctx, filter)
					if err != nil {
						return nil, err
					}

					return &EventIteratorWrapper{ch: ch}, nil
				}),
			},
			// "store": map[string]any{
			// 	"get": func(key string) any {
			// 		globalStore.mutex.Lock()
			// 		defer globalStore.mutex.Unlock()
			// 		return globalStore.data[key]
			// 	},
			// 	"set": func(key string, value any) {
			// 		globalStore.mutex.Lock()
			// 		globalStore.data[key] = value
			// 		globalStore.mutex.Unlock()
			// 	},
			// 	"del": func(key string) {
			// 		globalStore.mutex.Lock()
			// 		defer globalStore.mutex.Unlock()
			// 		delete(globalStore.data, key)
			// 	},
			// },
		},
	}
}

func makeConnectionObject(ctx context.Context) tengo.Object {
	return &tengo.Map{
		Value: map[string]tengo.Object{
			"get_ip": &tengo.UserFunction{
				Name: "get_ip",
				Value: tengo.CallableFunc(func(_ ...tengo.Object) (tengo.Object, error) {
					ip := khatru.GetIP(ctx)
					if ip == "" {
						return &tengo.Undefined{}, nil
					}
					return &tengo.String{Value: ip}, nil
				}),
			},
			"get_authed_pubkey": &tengo.UserFunction{
				Name: "get_authed_pubkey",
				Value: tengo.CallableFunc(func(_ ...tengo.Object) (tengo.Object, error) {
					pubkey := khatru.GetAuthed(ctx)
					if pubkey == "" {
						return &tengo.Undefined{}, nil
					}
					return &tengo.String{Value: pubkey}, nil
				}),
			},
			// "store": map[string]any{
			// 	"get": func(key string) any {
			// 		store, _ := sessionStorage.LoadOrCompute(khatru.GetConnection(ctx), func() store {
			// 			return store{data: make(map[string]any)}
			// 		})
			// 		store.mutex.Lock()
			// 		defer store.mutex.Unlock()
			// 		return store.data[key]
			// 	},
			// 	"set": func(key string, value any) {
			// 		store, _ := sessionStorage.LoadOrCompute(khatru.GetConnection(ctx), func() store {
			// 			return store{data: make(map[string]any)}
			// 		})
			// 		store.mutex.Lock()
			// 		store.data[key] = value
			// 		store.mutex.Unlock()
			// 	},
			// 	"del": func(key string) {
			// 		store, _ := sessionStorage.LoadOrCompute(khatru.GetConnection(ctx), func() store {
			// 			return store{data: make(map[string]any)}
			// 		})
			// 		store.mutex.Lock()
			// 		defer store.mutex.Unlock()
			// 		delete(store.data, key)
			// 	},
			// },
		},
	}
}
