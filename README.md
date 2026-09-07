# Blue

Blue is a collection of Go packages we have found useful across various
projects. There is no single "Blue" application: it is a grab bag of small,
independent libraries — document storage, iterators, CLI scaffolding, auth,
logging, releases — that we kept rewriting until we put them in one module.

```
go get github.com/unstablebuild/blue
```

Requires Go 1.26 or later.

## Packages

| Package | What it is |
| --- | --- |
| [`auth`](auth) | OAuth2 client flow with a local callback listener, JWT signing/verification, key sources (static, JWKS, GCP Secret Manager), HTTP middleware, and gRPC interceptors ([`grpcauth`](auth/grpcauth)). |
| [`bluectx`](bluectx) | `First(parent, ctxs...)` merges several contexts into one that cancels with the first of them. |
| [`bluenet`](bluenet) | `net.Conn` adapters over non-socket transports: channels (`ChanConn`) and reader/writer pairs (`PipeConn`, `StdioConn`). |
| [`cli`](cli) | A `flag`-based framework for hierarchical commands with generated usage and man pages, plus output formatters ([`cliformat`](cli/cliformat): table, JSON, template). |
| [`config`](config) | Thin wrapper over `go.uber.org/config` for layering a reference YAML literal with an override file. |
| [`crypto`](crypto) | PGP signing and verification helpers over `go-crypto/openpgp`. |
| [`debug`](debug) | `CapturePanic` turns a panic into an `issue.Report` crash report with stack trace and build info. |
| [`document`](document) | Backend-agnostic document store: a `Service` interface with filters, preconditions, and batching. Backends: in-memory, [`bolt`](document/bolt), [`firestore`](document/firestore), [`docrpc`](document/docrpc) (gRPC). Decorators for partitioning, filtering, syncing, and [logging](document/doclog). [`doctest`](document/doctest) is a reusable conformance suite. |
| [`emailprovider`](emailprovider) | Provider-neutral transactional email interface, with a dependency-free [SendGrid](emailprovider/sendgrid) v3 implementation. |
| [`issue`](issue) | Issue tracker interface and a `document`-backed implementation with auto-incrementing issue numbers. |
| [`iterator`](iterator) | Generic lazy iterators with `Map`, `Filter`, `Reduce`, and `Aggregate`, plus bridges from slices, gRPC streams, and `document.Iterator`. |
| [`logging`](logging) | Structured attempt/result logging conventions on `logrus`, with Cloud Logging and logd formatters, HTTP/gRPC middleware, and [trace](logging/trace) IDs. |
| [`release`](release) | Package and release artifact management. Metadata and blobs in a document store ([`docrelease`](release/docrelease)), bundles in GCS ([`gcsrelease`](release/gcsrelease)), read-only HTTP/CDN access ([`cdnrelease`](release/cdnrelease)), and PGP signing ([`signedrelease`](release/signedrelease)). |
| [`retry`](retry) | Composable retry strategies: limit, exponential, sequential, and combinations. |
| [`upspin`](upspin) | An `upspin.File` implementation that buffers the file in memory so it can be read and written at the same time. |

## Commands

### `bluectl`

`bluectl` is the CLI we use at Unstable Build to project manage
[Rune](https://rune.build): it tracks issues, publishes releases, manages
secrets, and sends newsletters.

```sh
go install github.com/unstablebuild/blue/cmd/bluectl@latest
```

```
$ bluectl -h
Manage internal resources.

Usage: bluectl [options] <cmd>

Options:
  -V           Run with verbose instrumentation. [default: false]
  -c           Use a different config folder. [default: ~/.bluectl]
  -h           Display this message [default: false]
  -v           Print CLI version information to stdout. [default: false]

Commands:
  release      Manage blue package releases
  package      Manage blue packages
  secret       Manage blue secrets
  analysis     Run Go linked package analysis against an executable
  issue        Manage blue's issue tracker
  newsletter   Print newsletter subscribers to stdout.
  init         Initialize or reinitialize this CLI's configuration.
  email        Render, preview, and send email from Go templates.
  license      License files by either updating, or adding the license header found in the given license file.
```

#### Backing services

`bluectl` is not self-contained. Except for `license` and `analysis`, every
command is a thin front end over Google Cloud and SendGrid, so it is only
useful once you have your own project to point it at:

- **Firestore** stores issues, package/release metadata, and newsletter
  subscribers, one collection per domain.
- **Cloud Storage** stores the release bundles themselves; Firestore only
  keeps the metadata.
- **Secret Manager** backs `bluectl secret`.
- **SendGrid** sends the mail composed by `bluectl email`.

Run `bluectl init` to create `~/.bluectl/config` (mode `0600`). It prompts
for the GCP project ID and writes the rest as defaults, which you then edit
by hand:

```yaml
auth:
  project-id: your-gcp-project
  credentials-file: /path/to/service-account.json
release:
  collection: coll-release   # Firestore collection
  bucket: coll-release       # GCS bucket, globally unique
issue:
  collection: coll-issue
password:
  collection: coll-secret
newsletter:
  collection: newsletter-subscribers
email:
  sender: you@example.com
  reply-to: you@example.com
  sendgrid:
    api-key: ...
    unsubscribe-group-id: 0
```

Backends are constructed lazily, so a command only pays for the credentials
it actually needs — `bluectl license` never touches the network. Use `-c` to
select a different config folder when you keep separate staging and
production settings.

### `bluebot`

A Slack bot that watches the issue collection and posts notifications when
reports change. See [`deploy/Dockerfile`](deploy/Dockerfile).

## Development

```sh
make            # build the binaries into bin/
make test       # run tests with the race detector
make lint       # golangci-lint
make license    # apply LICENSE_HEADER to all Go files
```

## License

Apache License 2.0. See [LICENSE](LICENSE).
