# p2p-storage-cluster
A p2p storage cluster for sharing using consistent hashing 



# Execution

 ```
 go run main.go
 ```

```
Welcome to p2p-storage-cluster
By Bits
Type help for a list of commands

chord> help

--- List of Chord Ring Commands ---
     port <name>       : Set the port the local node should listen on
     create            : Create a new ring
     join <address>    : Join an existing ring that has a node with <address> in it
     quit              : Shutdown the node. If this is the last node in
                         the ring, the ring also shuts down
--- Key/Value Operations ---
     put <key> <value> : Insert the <key> and <value> into the active ring
     putrandom <n>     : Generates <n> random keys and values and inserts them
                         into the active ring
     get <key>         : Find <key> in the active ring
     delete <key>      : Delete <key> from the active ring
--- Debugging Commands ---
     dump              : Display information about the current node
     dumpkey <key>     : Show information about the node that contains <key>
     dumpaddr <addr>   : Show information about the node at the given <addr>
     dumpall           : Show information about all nodes in the active ring
     ping <addr>       : Check if a node is listening on <addr>
```
