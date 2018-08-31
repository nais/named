named
=====

[![Build Status](https://travis-ci.org/nais/named.svg?branch=master)](https://travis-ci.org/nais/named)
[![Go Report Card](https://goreportcard.com/badge/github.com/nais/named)](https://goreportcard.com/report/github.com/nais/named)
[![License](http://img.shields.io/badge/license-mit-blue.svg?style=flat-square)](https://raw.githubusercontent.com/nais/named/master/LICENSE)

k8s in-cluster daemon with API for performing OpenAM-operations

Basic outline

1. HTTP POST to API with name of application and zone
2. Get and inject environment specific variables from Fasit
3. If SBS: Fetches app-policies.xml and not-enforced-urls.txt from internal artifact repository
4. Creates appropriate OpenAM configuration


#### Configure

```sh
named configure [flags]

Flags:
  -a, --app string            name of your app
  -c, --cluster string	      name of cluster you want to configure
  -r, --contexts string array list of context roots for ISSO agent
  -e, --environment string    environment you want to use (default "t0")
  -p, --password string       the password
  -u, --username string       the username
  -v, --version string        version you want to deploy
      --wait                  whether to wait until the deploy has succeeded (or failed)
```

The username and password may be specified using environment variable `NAIS_USERNAME` and `NAIS_PASSWORD` instead.
curl can also be used to initialize the service.


### Installation

Binaries for `amd64` Linux, Darwin and Windows are automatically released on every build.

The commands below will assume you have already [downloaded a release](https://github.com/nais/named/releases).


### Install Linux/macOS

```sh
xz -d named-<arch>-amd64.xz
mv named-<arch>-amd64 /usr/local/bin/name
chmod +x /usr/local/bin/name
```

Where `<arch>` will be `linux` or `darwin`.


### Windows

Unzip the release and place it somewhere.


## CI

on push:

- run tests
- produce binary
- bump version
- make and publish alpine docker image with binary to dockerhub
- make and publish corresponding helm chart to quay.io 


## Development

Fetch dependencies:
```dep ensure```

Build binary:
```go build -i .```

