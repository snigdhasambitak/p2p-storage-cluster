package src

import (
	"bytes"
	"fmt"
	"log"
	"strconv"
	"time"
	"math/big"
	"net"
	"net/http"
	"net/rpc"
)

//Size of this ring

var biggest *big.Int
var sendNothing Nothing
var returnNothing *Nothing
type Nothing struct{}

type Server struct {
	node     *Node
	listener net.Listener
	active   bool
	fx       chan func(*Node)
	i        int
}

type Node struct {
	ID          *big.Int          //160 bit integer
	Address     string            //Storage format: 192.168.0.1
	Port        string            //Storage format: 3410
	Predecessor string            //Storage format: 192.168.0.1:3409
	Successors  []string          //Storage format: 192.168.0.1:3411
	Fingers     []string          //Storage format: 192.168.0.1:3412
	Bucket      map[string]string //[key] = value
	next        int               //Keeps track of current finger
}

const (
	maxFingers    = 161
	maxSuccessors = 3
)

func CreateServer(n *Node) *Server {
	biggest = new(big.Int).Exp(big.NewInt(2), big.NewInt(160), big.NewInt(0))
	PrintPrompt("Creating RPC server for new node...")
	return &Server{node: n, fx: make(chan func(*Node)), i: 0}
}

func Listen(s *Server) {
	go s.recieverLoop()

	rpc.Register(s)
	rpc.HandleHTTP()
	PrintPrompt()
	fmt.Printf("RPC server is listening on port: %s\n", s.node.Port)
	l, e := net.Listen("tcp", ":"+s.node.Port)
	if e != nil {
		log.Fatal("Listen: Listen error:", e)
	}
	s.active = true
	go http.Serve(l, nil)
}

//Actor pattern - makes sure data is safe for concurrency
func (s *Server) recieverLoop() {
	for {
		select {
		case f := <-s.fx:
			f(s.node)
		}
	}
}


func (s *Server) Create(_ Nothing, _ *Nothing) error {
	finished := make(chan struct{})
	s.fx <- func(n *Node) {
		n.Predecessor = ""
		n.Successors[0] = net.JoinHostPort(n.Address, n.Port)
		finished <- struct{}{}
	}
	<-finished
	go s.keepCheckingPredecessor()
	go s.keepStabilizing()
	go s.keepFixingFingers()
	return nil
}

func CreateNode(port string) *Node {
	if port != "" {
		host := net.JoinHostPort(GetLocalAddress(), port)
		newID := HashString(host)
		address := GetLocalAddress()
		PrintPrompt("Creating new node...")
		fmt.Println("       ID:      " + newID.String())
		fmt.Println("       Address: " + address + ":" + port)
		return &Node{ID: newID,
			Address:     address,
			Port:        port,
			Predecessor: "",
			Successors:  make([]string, maxSuccessors),
			Fingers:     make([]string, maxFingers),
			Bucket:      make(map[string]string)}
	} else {
		PrintPrompt("CreateNode: Node not created - port not specified")
		return nil
	}
}

func (s *Server) Join(address string, reply *int) error {
	var pingReply *int
	*reply = 0
	successorSet := false
	PrintPrompt("Joining ring " + address + "...")
	finished := make(chan struct{})
	s.fx <- func(n *Node) {
		n.Predecessor = ""
		var nPrime *string
		Call(address, "Server.Ping", sendNothing, &pingReply)
		if pingReply != nil && *pingReply == 562 {
			Call(address, "Server.FindSuccessor", HashString(net.JoinHostPort(n.Address, n.Port)), &nPrime)
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
			s.TransferAll(sendNothing, returnNothing)
		}()
		*reply = 1
	} else {
		*reply = 0
	}
	return nil
}

func (s *Server) GetSuccessors(_ Nothing, successors *[]string) error {
	finished := make(chan struct{}, 1)
	s.fx <- func(n *Node) {
		*successors = n.Successors
		finished <- struct{}{}
	}
	<-finished
	return nil
}

func (s *Server) FindSuccessor(id *big.Int, reply *string) error {
	var nPrime string
	callFS := false
	finished := make(chan struct{})
	s.fx <- func(n *Node) {
		if Between(n.ID, id, HashString(n.Successors[0]), true) {
			*reply = n.Successors[0]
		} else {
			callFS = true
		}
		finished <- struct{}{}
	}
	<-finished
	if callFS {
		nPrime = s.closestPrecedingNode(id)
		Call(nPrime, "Server.FindSuccessor", id, &reply)
	}
	return nil
}

func (s *Server) closestPrecedingNode(id *big.Int) string {
	var result string
	between := false
	finished := make(chan struct{})
	s.fx <- func(n *Node) {
		for i := maxFingers - 1; i > 1; i-- {
			if Between(n.ID, HashString(n.Fingers[i]), id, false) {
				result = n.Fingers[i]
				between = true
			}
		}
		if !between {
			result = n.Successors[0]
		}
		finished <- struct{}{}
	}
	<-finished
	return result
}

func (s *Server) Ping(_ Nothing, reply *int) error {
	*reply = 562
	return nil
}

func (s *Server) Put(keyval []string, success *int) error {
	finished := make(chan struct{})
	s.fx <- func(n *Node) {
		n.Bucket[keyval[0]] = keyval[1]
		*success = 1
		finished <- struct{}{}
	}
	<-finished
	return nil
}

func (s *Server) PutAll(keyvals map[string]string, success *int) error {
	finished := make(chan struct{})
	s.fx <- func(n *Node) {
		for key, value := range keyvals {
			n.Bucket[key] = value
		}
		*success = 1
		finished <- struct{}{}
	}
	<-finished
	return nil
}

func (s *Server) Get(key string, reply *[]string) error {
	finished := make(chan struct{})
	s.fx <- func(n *Node) {
		for k, v := range n.Bucket {
			if k == key {
				*reply = append(*reply, k)
				*reply = append(*reply, v)
			}
		}
		finished <- struct{}{}
	}
	<-finished
	return nil
}

func (s Server) GetAll(_ Nothing, bucket *map[string]string) error {
	finished := make(chan struct{})
	s.fx <- func(n *Node) {
		*bucket = n.Bucket
		finished <- struct{}{}
	}
	<-finished
	return nil
}

func (s *Server) TransferAll(_ Nothing, _ *Nothing) error {
	var reply *int
	var sendNothing Nothing
	var deleteKeys []string
	var succ string
	var succBucket map[string]string

	finished := make(chan struct{})
	s.fx <- func(n *Node) {
		succ = n.Successors[0]
		finished <- struct{}{}
	}
	<-finished
	Call(succ, "Server.GetAll", sendNothing, &succBucket)

	finished2 := make(chan struct{})
	//Get all keys between this node and successor
	s.fx <- func(n *Node) {
		for key, value := range succBucket {
			if Between(n.ID, HashString(key), HashString(n.Successors[0]), false) {
				n.Bucket[key] = value
				deleteKeys = append(deleteKeys, key)
			}
		}
		finished2 <- struct{}{}
	}
	<-finished2
	//Now delete keys off successor that were added to this node so there are no duplicates
	for _, v := range deleteKeys {
		Call(succ, "Server.Delete", v, &reply)
	}
	return nil
}

func (s *Server) Delete(key string, success *int) error {
	finished := make(chan struct{})
	s.fx <- func(n *Node) {
		if _, ok := n.Bucket[key]; ok {
			delete(n.Bucket, key)
			*success = 1
		} else {
			*success = 0
		}

		finished <- struct{}{}
	}
	<-finished
	return nil
}

func (s *Server) Dump(_ Nothing, reply *string) error {
	finished := make(chan struct{})
	s.fx <- func(n *Node) {
		var buffer bytes.Buffer
		buffer.WriteString("\nRing Size:    " + biggest.String() + "\n")
		buffer.WriteString("Address:       " +
			HashString(n.Address+":"+n.Port).String() +
			" (" + net.JoinHostPort(n.Address, n.Port) + ")" + "\n")
		buffer.WriteString("Predecessor:   " + HashString(n.Predecessor).String() +
			" (" + n.Predecessor + ")\n")
		buffer.WriteString("Successors:    ")
		i := 0
		for _, v := range n.Successors {
			if len(v) > 0 {
				if i == 0 {
					buffer.WriteString(HashString(v).String() + " (" + v + ")\n")
				} else {
					buffer.WriteString("               " +
						HashString(v).String() + " (" + v + ")\n")
				}
			}
			i++
		}

		i = 0
		//Just show finger entries that are different
		buffer.WriteString("Fingers:")
		var diffFinger string
		for j := 1; j < maxFingers; j++ {
			if i == 0 {
				diffFinger = n.Fingers[j]
				buffer.WriteString(" [" + fmt.Sprintf("%3d", j) + "] " +
					HashString(n.Fingers[j]).String() +
					" (" + n.Fingers[j] + ")\n")
			} else if n.Fingers[j] != "" && n.Fingers[j] != diffFinger {
				diffFinger = n.Fingers[j]
				buffer.WriteString("         " + "[" + fmt.Sprintf("%3d", j) +
					"] " + HashString(n.Fingers[j]).String() +
					" (" + n.Fingers[j] + ")\n")
			}
			i++
		}

		i = 0
		buffer.WriteString("\nBucket:        ")
		for k, v := range n.Bucket {
			if i == 0 {
				buffer.WriteString("[" + k + "]: " + v + "\n")
			} else {
				buffer.WriteString("               [" + k + "]: " + v + "\n")
			}
			i++
		}
		buffer.WriteString("Items in bucket: " + strconv.Itoa(len(n.Bucket)) + "\n")
		*reply = buffer.String()
		finished <- struct{}{}
	}
	<-finished
	return nil
}

func (s *Server) keepStabilizing() {
	interval := time.Tick(1 * time.Second)
	for {
		select {
		case <-interval:
			s.stabilize()
		}
	}
}

func (s *Server) stabilize() {
	var reply, alive *int
	var nothing Nothing
	var nodeSucc, newSucc, currNode string
	var succPred *string
	var succSnap, succSuccs []string
	nodeSucc = ""
	succSnap = make([]string, maxSuccessors)

	finished := make(chan struct{}, 1)
	s.fx <- func(n *Node) {
		//Take a snapshot of successor list to determine if it has changed
		//when this function finishes
		copy(succSnap, n.Successors)
		nodeSucc = n.Successors[0]
		finished <- struct{}{}
	}
	<-finished

	Call(nodeSucc, "Server.Ping", nothing, &alive)

	finished2 := make(chan struct{}, 1)
	s.fx <- func(n *Node) {
		//Successor node has failed
		if alive == nil {
			succSuccs = n.Successors
			succSuccs = succSuccs[1:]
			if len(succSuccs) <= 1 {
				succSuccs = append(succSuccs, n.Address+":"+n.Port)
			}
			n.Successors = succSuccs
		}
		finished2 <- struct{}{}
	}
	<-finished2

	Call(nodeSucc, "Server.GetPredecessor", nothing, &succPred)
	Call(nodeSucc, "Server.GetSuccessors", nothing, &succSuccs)

	finished3 := make(chan struct{}, 1)
	s.fx <- func(n *Node) {
		if succPred != nil && *succPred != "" &&
			Between(n.ID, HashString(*succPred), HashString(n.Successors[0]), false) {
			n.Successors[0] = *succPred
		}

		//Create list of successors
		if len(succSuccs) >= maxSuccessors {
			succSuccs = succSuccs[:len(succSuccs)-1]
		}
		succSuccs = append([]string{n.Successors[0]}, succSuccs...)
		n.Successors = n.Successors[:0]
		n.Successors = succSuccs

		newSucc = n.Successors[0]
		currNode = net.JoinHostPort(n.Address, n.Port)
		if succSnap[0] != n.Successors[0] ||
			succSnap[1] != n.Successors[1] ||
			succSnap[2] != n.Successors[2] {
			fmt.Printf("Stabilize: Successor list changed\n")
			PrintPrompt()
		}
		finished3 <- struct{}{}
	}
	<-finished3
	Call(newSucc, "Server.Notify", currNode, &reply)
}

func (s *Server) GetPredecessor(_ Nothing, predaddress *string) error {
	finished := make(chan struct{}, 1)
	s.fx <- func(n *Node) {
		*predaddress = n.Predecessor
		finished <- struct{}{}
	}
	<-finished
	return nil
}

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

func (s *Server) keepCheckingPredecessor() {
	interval := time.Tick(1 * time.Second)
	for {
		select {
		case <-interval:
			s.checkPredecessor()
		}
	}
}

func (s *Server) checkPredecessor() error {
	finished := make(chan struct{})
	s.fx <- func(n *Node) {
		if n.Predecessor != "" {
			var nothing Nothing
			var predreply *int
			//Dial the predecessor
			Call(n.Predecessor, "Server.Ping", nothing, &predreply)
			if predreply == nil || *predreply != 562 {
				n.Predecessor = ""
			}
		}
		finished <- struct{}{}
	}
	<-finished
	return nil
}

func (s *Server) keepFixingFingers() {
	interval := time.Tick(1 * time.Second)
	for {
		select {
		case <-interval:
			s.fixFingers()
		}
	}
}

func (s *Server) fixFingers() error {
	var address string
	var next int
	reply := ""
	finished := make(chan struct{})
	s.fx <- func(n *Node) {
		n.Fingers[1] = n.Successors[0]
		n.next = n.next + 1
		if n.next > keySize {
			n.next = 1
		}
		address = net.JoinHostPort(n.Address, n.Port)
		next = n.next
		finished <- struct{}{}
	}
	<-finished
	s.FindSuccessor(jump(address, next), &reply)
	finished2 := make(chan struct{})
	s.fx <- func(n *Node) {
		n.Fingers[n.next] = reply
		for n.next+1 < keySize &&
			Between(n.ID, jump(address, n.next+1), HashString(reply), false) {
			n.next += 1
			n.Fingers[n.next] = reply
		}
		finished2 <- struct{}{}
	}
	<-finished2
	return nil
}