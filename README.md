jingle
======

A Nostr relay that can be easily customized with The JavaScript.

## How to run it

If you have a Go compiler you can install it with

```
go install github.com/fiatjaf/jingle@latest
```

Otherwise you can get a binary from the [releases](../../releases) page and then calling `chmod +x ...` on it.

Then run it from the shell with `jingle` (or whatever is your binary called):

```
~> jingle
INF checking for scripts under ./stuff/
INF storing data with sqlite under ./data/sqlite
INF checking for html and assets under ./stuff/
INF running on http://0.0.0.0:5577
```

This will create a `./data` and a `./stuff` directories under the current directory.

- `./data` is where your database will be placed, with all the Nostr events and indexes. By default it will be an SQLite database under `./data/sqlite`, but you can also specify `--db lmdb` or `--db badger` to use different storage mechanisms.
- `./stuff` is where you should define your custom rules for rejecting events or queries and subscriptions. Two example files will be created and are intended to be modified. They can be modified at runtime.
  - `reject-event.js`: this should `export default` a function that takes every incoming `event` as a parameter and returns a string with an error message when that event should be rejected and returns `null` or `undefined` when the event should be accepted. It can also return a `Promise` that resolves to one of these things.
  - `reject-filter.js`: this is the same, but takes a `filter` as a parameter and should return an error string if that filter should be rejected.
  - `index.html` and other `.html` files: these will be served under the root of your relay HTTP server, if present, but they are not required.

### Other options

Call `jingle --help` to see other possible options. All of these can also be set using environment variables. The most common ones will probably be `--name`, `--pubkey`, `--icon` and `--description`, used to set basic NIP-11 metadata for the relay.

### Trying it

Since you are already in the command line you can download https://github.com/fiatjaf/nak and try writing some events or queries to your relay.
