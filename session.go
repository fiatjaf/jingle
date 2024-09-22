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
	data  map[string]tengo.Object
	mutex sync.Mutex
}

var globalStore = store{data: make(map[string]tengo.Object)}

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
			"store": &tengo.Map{
				Value: map[string]tengo.Object{
					"get": &tengo.UserFunction{
						Name: "store.get",
						Value: tengo.CallableFunc(func(args ...tengo.Object) (tengo.Object, error) {
							if len(args) < 1 {
								return nil, fmt.Errorf("store.get() needs an argument")
							}
							key := args[0].String()
							globalStore.mutex.Lock()
							defer globalStore.mutex.Unlock()
							return globalStore.data[key], nil
						}),
					},
					"set": &tengo.UserFunction{
						Name: "store.set",
						Value: tengo.CallableFunc(func(args ...tengo.Object) (tengo.Object, error) {
							if len(args) < 2 {
								return nil, fmt.Errorf("store.get() needs two arguments")
							}
							key := args[0].String()
							globalStore.mutex.Lock()
							globalStore.data[key] = args[1]
							globalStore.mutex.Unlock()
							return nil, nil
						}),
					},
					"del": &tengo.UserFunction{
						Name: "store.del",
						Value: tengo.CallableFunc(func(args ...tengo.Object) (tengo.Object, error) {
							if len(args) < 1 {
								return nil, fmt.Errorf("store.get() needs an argument")
							}
							key := args[0].String()
							globalStore.mutex.Lock()
							defer globalStore.mutex.Unlock()
							delete(globalStore.data, key)
							return nil, nil
						}),
					},
				},
			},
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
			"store": &tengo.Map{
				Value: map[string]tengo.Object{
					"get": &tengo.UserFunction{
						Name: "store.get",
						Value: tengo.CallableFunc(func(args ...tengo.Object) (tengo.Object, error) {
							if len(args) < 1 {
								return nil, fmt.Errorf("store.get() needs an argument")
							}
							key := args[0].String()
							store, _ := sessionStorage.LoadOrCompute(khatru.GetConnection(ctx), func() store {
								return store{data: make(map[string]tengo.Object)}
							})
							store.mutex.Lock()
							defer store.mutex.Unlock()
							return store.data[key], nil
						}),
					},
					"set": &tengo.UserFunction{
						Name: "store.set",
						Value: tengo.CallableFunc(func(args ...tengo.Object) (tengo.Object, error) {
							if len(args) < 2 {
								return nil, fmt.Errorf("store.get() needs two arguments")
							}
							key := args[0].String()
							store, _ := sessionStorage.LoadOrCompute(khatru.GetConnection(ctx), func() store {
								return store{data: make(map[string]tengo.Object)}
							})
							store.mutex.Lock()
							store.data[key] = args[1]
							store.mutex.Unlock()
							return nil, nil
						}),
					},
					"del": &tengo.UserFunction{
						Name: "store.del",
						Value: tengo.CallableFunc(func(args ...tengo.Object) (tengo.Object, error) {
							if len(args) < 1 {
								return nil, fmt.Errorf("store.get() needs an argument")
							}
							key := args[0].String()
							store, _ := sessionStorage.LoadOrCompute(khatru.GetConnection(ctx), func() store {
								return store{data: make(map[string]tengo.Object)}
							})
							store.mutex.Lock()
							defer store.mutex.Unlock()
							delete(store.data, key)
							return nil, nil
						}),
					},
				},
			},
		},
	}
}
