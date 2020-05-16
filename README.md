# micron

A minimal implementation of the fault-tolerant job scheduler based on distributed locks.


## Features

1. Fault-tolerant

    No single point of failure, just setup multiple Cron instances on different machines.

2. Exactly once

    The same job, although scheduled simultaneously by multiple Cron instances, is guaranteed to be executed exactly once per execution time of its schedule.

3. Embeddable

    No need to run standalone binaries, you can just embed Cron instances into your existing application, which usually consists of multiple nodes.


## Installation

```bash
$ go get -u github.com/RussellLuo/micron
```


## Example

See [example](example).


## Documentation

Checkout the [Godoc][1].


## License

[MIT](LICENSE)


[1]: https://godoc.org/github.com/RussellLuo/micron
