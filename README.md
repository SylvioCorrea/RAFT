# An implementation of Raft
An ongoing implementation of the Raft consensus protocol using go.

Usage:
`go run raft_server_start.go n`
where `n` is a unique server id from 0 to 2. Run 3 instances (0, 1, 2) in different terminals.

The server network is defined in the network.txt file. More servers can be added by modifying that file.