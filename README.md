jingle
======

A Nostr relay that can be easily customized with a nice simple dynamic language: [Tengo](https://tengolang.com/).

## How to run it

If you have a Go compiler you can install it with

```
go install github.com/fiatjaf/jingle@latest
```

```
~> jingle
INF checking for scripts under ./stuff/
INF storing data with sqlite under ./data/sqlite
INF checking for html and assets under ./stuff/
INF running on http://0.0.0.0:5577
```

This will create a `./data` and a `./stuff` directories under the current directory.

- `./data` is where your database will be placed, with all the Nostr events and indexes. By default it will be an SQLite database under `./data/sqlite`, but you can also specify `--db lmdb` or `--db badger` to use different storage mechanisms.
- `./stuff` is where you should define your custom rules for rejecting events or queries and subscriptions. 2 Tengo files will be created with example code in them, they are intended to be modified without having to restart the server. Other files can also be put in this directory. These are the possibilities:
  - `reject-event.tengo`: this file should `export default` a function that is called on every `EVENT` message received should return a string with an error message when that event should be rejected and `undefined` when the event should be accepted.
  - `reject-filter.tengo`: same as above, but refers to `REQ` messages instead.
  - `index.html` and other `.html` files: these will be served under the root of your relay HTTP server, if present, but they are not required.
  - `icon.png`, `icon.jpg` or `icon.gif`, if present, will be used as the relay NIP-11 icon.

### More about `reject-event.tengo` and `reject-filter.tengo`

**Function parameters**

They both take 3 parameters, in the following order:
  - `event`: the event being written, for `reject-event.tengo`; or `filter`: the subscription filter, for `reject-filter.tengo`.
  - `relay`: an object with some fields:
    - `query()`, a function that can be called with any Nostr filter and will return an array of results with events (read from the local database)
    - `store`, an interface for storing ephemeral data (will be stored in memory and cleaned up when the server stops), provides these functions:
      - `get(key)`
      - `set(key, value)`
      - `del(key)`
  - `conn`: an object with some fields:
    - `get_ip()`, the IP address of the user, as a string
    - `get_authed_pubkey()`, the public key of the user, as hex, if the user has performed authentication, otherwise `undefined`
    - `store`, an interface for storing data associated with this connection, provides these functions:
      - `get(key)`
      - `set(key, value)`
      - `del(key)`

**Authentication requests**

The functions can prompt a client to authenticate using the NIP-42 flow anytime by return a string that starts with `"auth-required: "` (and then some human-readable message afterwards). If the client performs an authentication and make a new request the `pubkey` will be set in the `conn` parameter.

**Tengo basics**

Tengo is a very simple language, as you can see here: https://tengolang.com/

It comes with these built-in functions: https://github.com/d5/tengo/blob/master/docs/builtins.md

It also comes with a standard library that you can use with `import("<module-name>")` calls: https://github.com/d5/tengo/blob/master/docs/stdlib.md

Besides these, we also ship an `http` module that can be imported in the same way. Currently it provides these functions:

  - `http.get("<url>")` -> returns a `string`

### Other options

Call `jingle --help` to see other possible options. All of these can also be set using environment variables. The most common ones will probably be `--name`, `--pubkey` and `--description`, used to set basic NIP-11 metadata for the relay.

### Trying it

Since you are already in the command line you can download https://github.com/fiatjaf/nak and try writing some events or queries to your relay.

## License

Public domain.
