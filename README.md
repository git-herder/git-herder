# Git Herder

Git Herder (`githerd`) is an open-source command-line tool for bulk
operations on git repositories. It works with repositories on SCM
platforms such as GitHub and GitLab, and with local working copies of
those repositories. It takes the routine out of large repository sets in
microservice and modular systems.

This repository is the Go module `github.com/git-herder/git-herder`.
Command behavior, flags, and exit codes are specified in the
documentation linked below. This tree is the implementation.

## What it does

| Command | What it does |
| --- | --- |
| `profile` | Create, show, edit, and delete named YAML profiles |
| `report` | List repositories selected on the SCM; the SCM is left unchanged |
| `clone` | Clone the selected repositories into `--workspace` with the host `git` |
| `sync` | Pull and push one branch across working copies already on it |
| `switch` | Switch selected working copies to one branch |
| `merge` | Merge a local branch into another across working copies |
| `publish` | For unpublished local copies, create the SCM repository, add a remote, and push |

`githerd --help` prints the command tree registered in the binary.

## Documentation

| Topic | Entry |
| --- | --- |
| Why the product exists | [Product intent](https://github.com/git-herder/documentation-product-intent/blob/master/README.md) |
| CLI and platform contracts | [Specifications](https://github.com/git-herder/documentation-specifications/blob/master/README.md) |
| Global flags, exit codes, version | [CLI contract](https://github.com/git-herder/documentation-specifications/blob/master/cli/index.md) |
| `profile` | [profile](https://github.com/git-herder/documentation-specifications/blob/master/cli/profile/index.md) |
| `report` | [report](https://github.com/git-herder/documentation-specifications/blob/master/cli/report/index.md) |
| `clone` | [clone](https://github.com/git-herder/documentation-specifications/blob/master/cli/clone/index.md) |
| `sync` | [sync](https://github.com/git-herder/documentation-specifications/blob/master/cli/sync/index.md) |
| `switch` | [switch](https://github.com/git-herder/documentation-specifications/blob/master/cli/switch/index.md) |
| `merge` | [merge](https://github.com/git-herder/documentation-specifications/blob/master/cli/merge/index.md) |
| `publish` | [publish](https://github.com/git-herder/documentation-specifications/blob/master/cli/publish/index.md) |
| Application structure | [Architecture](https://github.com/git-herder/documentation-architecture/blob/master/README.md) |
| Recorded technical choices | [ADRs](https://github.com/git-herder/ADR) |
| Terms | [Glossary](https://github.com/git-herder/glossary/blob/master/index.md) |

## Requirements

- Go. The project supports the two latest minor releases. The `go`
  directive in `go.mod` is the minimum this checkout builds with.
- `git` on `PATH` for `clone`, `sync`, `switch`, `merge`, and `publish`.
  The binary runs that `git` and does not embed one.

## Build

From a clone of this repository:

```bash
go build -o githerd ./cmd/githerd
```

## Install

A published version, into `$(go env GOPATH)/bin`:

```bash
go install github.com/git-herder/git-herder/cmd/githerd@latest
```

The same command from this clone:

```bash
go install ./cmd/githerd
```

Confirm the binary:

```bash
githerd --version
```
