# micron

A minimal implementation of the fault-tolerant job scheduler.


## Features

1. **Fault-tolerant**

    No single point of failure, just setup multiple Cron instances on different machines.

2. **Exactly-once**

    The same job, although scheduled simultaneously by multiple Cron instances, is guaranteed to be executed exactly once per execution time of its schedule.

    This guarantee is provided by leveraging distributed locks. The default implementation is based on Redis [with a single instance][1]. For higher safety and reliability, you can use other Raft-based implementations of distributed locks (e.g. [Consul][2], [etcd][3]).

3. **Embeddable**

    No need to run standalone binaries, you can just embed Cron instances into your existing application, which may consist of one or multiple nodes.


## Installation

```bash
$ go get -u github.com/RussellLuo/micron
```


## Example

See [example](example).


## Documentation

Checkout the [Godoc][4].


## License

[MIT](LICENSE)


[1]: https://redis.io/topics/distlock#correct-implementation-with-a-single-instance
[2]: https://www.consul.io/
[3]: https://etcd.io/
[4]: https://pkg.go.dev/mod/github.com/RussellLuo/micron
