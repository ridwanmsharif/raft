# raft

Distributed key-value store atop an implementation of the Raft Consensus Algorithm - a distributed consensus protocol for managing replicated state macines over unreliable networks.
This implementation implements an basic design for log truncations, replications, snapshots, leader elections and more.
It is designed to be a fault tolerant and performant alternative to the Paxos Consensus Algorithm. It is tolerant to network partitions and node failures.
Written primarily in Go, it uses protocol buffers with gRPC for the underlying RPC subsystem.

## Disclaimer

Purely experimental project. Designed for learning purposes not production use. Based on [research paper](https://raft.github.io/raft.pdf) Diego Ongaro et. al. Also inspired by [CoreOS/etcd/raft](github.com/coreos/raft). Improve functionality at will.

## Contributing

Bug reports and pull requests are welcome on GitHub at [@ridwanmsharif](https://www.github.com/ridwanmsharif)

## Author

Ridwan M. Sharif: [E-mail](mailto:ridwanmsharif@hotmail.com), [@ridwanmsharif](https://www.github.com/ridwanmsharif)

## License

The Library is available as open source under the terms of
the [MIT License](https://opensource.org/licenses/MIT)
