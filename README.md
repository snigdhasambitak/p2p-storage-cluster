# p2p-storage-cluster
A p2p storage cluster for sharing using consistent hashing 

# Implementation Details

The Chord protocol can support one operation: given a key, it will determine the node responsible for storing the key’s value. Chord does not itself store keys and values, but provides primitives that allow higher-layer software to build a wide variety of storage system; CFS is one such use of the Chord primitive.
We’ll try to explain the basic implementation of  a p2p storage system

## Querying mechanism 
   1. The core usage of the Chord protocol is to query a key from a client (generally a node as well), i.e. to find successor(k). 
   2. The basic approach is to pass the query to a node’s successor, if it cannot find the key locally. 
   3. This will lead to a `O(N)` query time where N is the number of machines in the ring. 
   4. To avoid the linear search above, Chord implements a faster search method by requiring each node to keep a finger table containing up to m entries, recall that m is the number of bits in the hash key 
   5. The i{th} entry of node n will contain successor `((n+2^{i-1}) mod 2^m)`. The first entry of finger table is actually the node’s immediate successor (and therefore an extra successor field is not needed). Every time a node wants to look up a key k, it will pass the query to the closest successor or predecessor (depending on the finger table) of k in its finger table (the “largest” one on the circle whose ID is smaller than k), until a node finds out the key is stored in its immediate successor. With such a finger table, the number of nodes that must be contacted to find a successor in an N-node network is `O(log N)`

![chordDHT](https://github.com/snigdhasambitak/p2p-storage-cluster/blob/main/images/chordDHT.jpeg)

```go
var two = big.NewInt(2)
var hashMod = new(big.Int).Exp(big.NewInt(2), big.NewInt(keySize), nil)

//Calculate exact position on chord ring (1/2, 1/4, 1/8, ...)  based on the fingertable entry
func jump(address string, fingerentry int) *big.Int {
	n := HashString(address)
	fingerentryminus1 := big.NewInt(int64(fingerentry) - 1)
	jump := new(big.Int).Exp(two, fingerentryminus1, nil)
	sum := new(big.Int).Add(n, jump)
	return new(big.Int).Mod(sum, hashMod)
}

//Sha-1 hashes a string
func HashString(elt string) *big.Int {
	hasher := sha1.New()
	hasher.Write([]byte(elt))
	return new(big.Int).SetBytes(hasher.Sum(nil))
}


```
The notify condition
```go
func (s *Server) Notify(nprime string, reply *int) error {
	finished := make(chan struct{}, 1)
	s.fx <- func(n *Node) {
		if n.Predecessor == "" ||
			Between(HashString(n.Predecessor), HashString(nprime), n.ID, false) {
			n.Predecessor = nprime
			fmt.Printf("Notify: predecessor set to: %s\n", n.Predecessor)
			PrintPrompt()
			*reply = 1
		} else {
			*reply = 0
		}
		finished <- struct{}{}
	}
	<-finished
	return nil
}
```
## Join condition of a Node

Whenever a new node joins, three invariants should be maintained (the first two ensure correctness and the last one keeps querying fast):

1. Each node’s successor points to its immediate successor correctly.
2. Each key is stored in successor(k)
3. Each node’s finger table should be correct.

To satisfy these invariants, a predecessor field is maintained for each node.

As the successor is the first entry of the finger table, we do not need to maintain this field separately any more. The following tasks should be done for a newly joined node n:

1. Initialize node n (the predecessor and the finger table).
2. Notify other nodes to update their predecessors and finger tables.
3. The new node takes over its responsible keys from its successor

The predecessor of n can be easily obtained from the predecessor of successor(n) (in the previous circle). As for its finger table, there are various initialization methods. The simplest one is to execute find successor queries for all m entries, resulting in O(M\log N) initialization time. A better method is to check whether i{th} entry in the finger table is still correct for the (i+1){th} entry. This will lead to O(log² N).

```go
func (s *Server) GetPredecessor(_ Nothing, predaddress *string) error {
	finished := make(chan struct{}, 1)
	s.fx <- func(n *Node) {
		*predaddress = n.Predecessor
		finished <- struct{}{}
	}
	<-finished
	return nil
}
```


## Stabilization Mechanism

To ensure correct lookups, all successor pointers must be up to date. Therefore, a stabilization protocol is running periodically in the background which updates finger tables and successor pointers.

The stabilization protocol works as follows:

1. Stabilize(): n asks its successor for its predecessor p and decides whether p should be n‘s successor instead (this is the case if p recently joined the system).
2. Notify(): notifies n‘s successor of its existence, so it can change its predecessor to n
3. fixFingers(): updates finger tables/*
4. checkPredecessor(): Periodically checks in predecessor is alive

```go
func (s *Server) Join(address string, reply *int) error {
	var pingReply *int
	*reply = 0
	successorSet := false
	PrintPrompt("Joining ring " + address + "...")
	finished := make(chan struct{})
	s.fx <- func(n *Node) {
		n.Predecessor = ""
		var nPrime *string
		err := Call(address, "Server.Ping", sendNothing, &pingReply)
		if err != nil {
			return 
		}
		if pingReply != nil && *pingReply == 562 {
			err := Call(address, "Server.FindSuccessor", HashString(net.JoinHostPort(n.Address, n.Port)), &nPrime)
			if err != nil {
				return 
			}
			n.Successors[0] = *nPrime
			successorSet = true
		} else {
			PrintPrompt("Address specified for join could not be contacted")
		}
		finished <- struct{}{}
	}
	<-finished
	if successorSet {
		go s.keepCheckingPredecessor()
		go s.keepStabilizing()
		go s.keepFixingFingers()

		go func() {
			time.Sleep(4 * time.Second)
			err := s.TransferAll(sendNothing, returnNothing)
			if err != nil {
				return 
			}
		}()
		*reply = 1
	} else {
		*reply = 0
	}
	return nil
}

```
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

# Results

The video attaches is a good description of the transfer mechanism and as you can see, when one node is down, the keys from that node get transferred to the successor node.

