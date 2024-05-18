# go-ra

[![Go Reference](https://pkg.go.dev/badge/github.com/YutaroHayakawa/go-ra.svg)](https://pkg.go.dev/github.com/YutaroHayakawa/go-ra)

Provides the `ra` package that implements a router-side functionality of IPv6
Neighbor Discovery mechanism
([RFC4861](https://datatracker.ietf.org/doc/html/rfc4861) and related RFCs). It
also provides a stand-alone daemon `gorad` and CLI tool `gora` to interact with
it. While the existing
[mdlayher/ndp](https://pkg.go.dev/github.com/mdlayher/ndp) package provides a
low-level protocol functionalities (packet encoding, raw-socket wrapper, etc),
`go-ra` implements an unsolicited and solicited advertisement machinery and
declarative configuration interface on top of it.

## Features

- Basic RA mechanism defined in RFC4861
- Router MAC address discovery with Source Link Layer Address option
- MTU discovery with MTU option
- Prefix discovery with Prefix Information option
- TBD: DNS configuration discovery with RDNSS/DNSSL option
- TBD: Route advertisement with Route Information option

## Installation

- Library: Use Go Modules as usual
- Stand-alone Binary: Visit [release page](https://github.com/YutaroHayakawa/go-ra/releases) and install pre-build binaries

## Basic Usage

### As a library

```go
// Build a configuration
config := ra.Config{
	  Interfaces: []*ra.InterfaceConfig{
		    {
			      Name: "eth0",
			      // Send unsolicited RA once a second
			      RAIntervalMilliseconds: 1000, // 1sec
		    },	
	  },
}

// Create an RA daemon
daemon, _ := ra.NewDaemon(&config)

// Run it
ctx, cancel := context.WithCancel(context.Background())
go daemon.Run(ctx)

// Get a running status
status := daemon.Status()
for _, iface := range status.Interfaces {
    fmt.Printf("%s: %s (%s)\n", iface.Name, iface.State, iface.Message)
}

// Change configuration and reload
config.Interfaces[0].RAIntervalMilliseconds = 2000 // 2sec
err := daemon.Reload(ctx, &config)
if err != nil {
    panic(err)
}

// Stop it
cancel()
```

### As a stand-alone daemon

Create a configuration file. This configuration will be translated into the
[Config](https://pkg.go.dev/github.com/YutaroHayakawa/go-ra#Config) object
and passed to the daemon. Please see the godoc for more details.

```yaml
interfaces:
- name: eth0
  raIntervalMilliseconds: 1000 # 1sec
```

Start daemon. You need a root privilege to run it.

```bash

$ sudo gorad -f config.yaml
```

Get status.

```bash
$ gora status
Name         Age    TxUnsolicited    TxSolicited    State      Message
eth0         22s    21               1              Running
```

Modify and reload configuration

```bash
$ gora reload -f config.yaml
```

## Motivation

Our original motivation for this project was use it with
[gobgp](https://pkg.go.dev/github.com/osrg/gobgp/v3) library to do [BGP
Unnumbered](https://github.com/osrg/gobgp/blob/master/docs/sources/unnumbered-bgp.md)
(see our [integration test](integration_tests/gobgp_unnumbered_test.go)) which
for us, makes sense to reinvent the RA daemon to not introduce an external
non-go dependency.
