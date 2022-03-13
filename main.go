package main

import (
	"bufio"
	"bytes"
	"fmt"
	chord "github.com/snigdhasambitak/p2p-storage-cluster/src"
	"log"
	"math/big"
	"math/rand"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

type Nothing struct{}

var sendNothing chord.Nothing
var returnNothing *chord.Nothing

var port string
var Node *chord.Node
var Server *chord.Server
var serverRunning, ringJoined bool

func main() {
	if len(os.Args) != 1 {
		log.Fatal("Number of command line arguments provided is incorrect")
	}
	serverRunning := false
	ringJoined := false
	rand.Seed(time.Now().UTC().UnixNano())

	fmt.Println("\nWelcome to Chord v1.0")
	fmt.Println("By Shawn Wonder")
	fmt.Println("Type help for a list of commands\n")
	chord.PrintPrompt()

	//Start reading lines of text that the user inputs
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		command := scanner.Text()
		commandTokens := strings.Fields(command)
		if len(commandTokens) > 0 {
			//Specifiy port that this node will listen on - port <port>
			if commandTokens[0] == "port" {
				if !serverRunning && !ringJoined {
					if len(commandTokens) > 2 {
						chord.PrintPrompt("Number of arguments supplied incorrect - usage: port <port>")
					} else if len(commandTokens) == 2 {
						port = commandTokens[1]
					} else {
						port = "3410"
					}
					if v, _ := strconv.Atoi(port); v > 0 {
						chord.PrintPrompt("Port set to: " + port)
					}
				} else {
					chord.PrintPrompt("Port cannot be set - create or join command already issued")
				}
				//Create a new ring
			} else if commandTokens[0] == "create" {
				if port == "" {
					chord.PrintPrompt("Please set the port before creating ring")
				} else if serverRunning {
					chord.PrintPrompt("Server cannot be created - server already running")
				} else if ringJoined {
					chord.PrintPrompt("Server cannot be created - already joined existing ring")
				} else {
					Node = chord.CreateNode(port)
					Server = chord.CreateServer(Node)
					chord.Listen(Server)
					Server.Create(sendNothing, returnNothing)
					serverRunning = true
				}
				//Join an existing ring - join <addr>
			} else if commandTokens[0] == "join" {
				if !serverRunning && !ringJoined {
					joinSuccess := 0
					Node = chord.CreateNode(port)
					if Node != nil {
						Server = chord.CreateServer(Node)
						chord.Listen(Server)
						Server.Join(commandTokens[1], &joinSuccess)
						ringJoined = true
					} else {
						chord.PrintPrompt("Node has not been created")
					}
					if ringJoined {
						chord.PrintPrompt("Now part of ring: " + commandTokens[1])
					}
				} else {
					chord.PrintPrompt("Ring cannot be joined - create or join command already issued")
				}
				//Ping a node to see if it is listening - ping <address>
			} else if commandTokens[0] == "ping" {
				if len(commandTokens) == 2 {
					var reply *int
					chord.Call(commandTokens[1], "Server.Ping", sendNothing, &reply)
					if reply != nil && *reply == 562 {
						chord.PrintPrompt("Response recieved from " + commandTokens[1])
					} else {
						chord.PrintPrompt("No response from " + commandTokens[1])
					}
				} else {
					chord.PrintPrompt("Number of arguments supplied incorrect - usage: ping <addr>")
				}
				//Insert key and value into the active ring - put <key> <value>
			} else if commandTokens[0] == "put" {
				if len(commandTokens) == 3 {
					var bucket []string
					var fsReply *string
					var reply *int
					hashedKey := chord.HashString(commandTokens[1])
					//Determine which node this key should be placed in
					chord.Call(net.JoinHostPort(chord.GetLocalAddress(), port), "Server.FindSuccessor", hashedKey, &fsReply)

					bucket = append(bucket, commandTokens[1])
					bucket = append(bucket, commandTokens[2])
					//Place key-value in the correct node
					chord.Call(*fsReply, "Server.Put", bucket, &reply)
					chord.PrintPrompt("[" + commandTokens[1] + "] = " + commandTokens[2] + " added")
				} else {
					chord.PrintPrompt("Number of arguments supplied incorrect - usage: put <key> <value>")
				}
				//Insert n active random keys and values into the active ring - putrandom <n>
			} else if commandTokens[0] == "putrandom" {
				if len(commandTokens) == 2 {
					n, _ := strconv.Atoi(commandTokens[1])
					if n > 0 {
						keysAdded := 0
						var bucket []string
						var fsReply *string
						var reply *int
						//Generate n random keys and values
						randKeyVals := make(map[string]string)
						for i := 0; i < n; i++ {
							randKeyVals[chord.RandString(10, false, false)] = chord.RandString(12, true, true)
						}
						var hashedKey *big.Int
						//Loop through each random key value and get node it should be placed in
						for k, v := range randKeyVals {
							bucket = bucket[:0]
							hashedKey = chord.HashString(k)
							chord.Call(net.JoinHostPort(chord.GetLocalAddress(), port), "Server.FindSuccessor", hashedKey, &fsReply)
							bucket = append(bucket, k)
							bucket = append(bucket, v)
							//Send the key-value pair to node
							chord.Call(*fsReply, "Server.Put", bucket, &reply)
							if *reply == 1 {
								keysAdded += 1
							}
						}
						chord.PrintPrompt(strconv.Itoa(keysAdded) + " random key values added")
					} else {
						chord.PrintPrompt("Number of keys to add must be greater than 0")
					}

				} else {
					chord.PrintPrompt("Number of arguments supplied incorrect - usage: putrandom <n>")
				}
				//Find key in the active ring - get <key>
			} else if commandTokens[0] == "get" {
				if len(commandTokens) == 2 {
					var fsReply *string
					var reply []string
					hashedKey := chord.HashString(commandTokens[1])
					chord.Call(net.JoinHostPort(chord.GetLocalAddress(), port), "Server.FindSuccessor", hashedKey, &fsReply)
					chord.Call(*fsReply, "Server.Get", commandTokens[1], &reply)
					if reply != nil {
						chord.PrintPrompt("Key value pair found: [" + reply[0] + "] = " + reply[1])
					} else {
						chord.PrintPrompt("Key was not found")
					}
				} else {
					chord.PrintPrompt("Number of arguments supplied incorrect - usage: get <key>")
				}
				//Delete key from the active ring - delete <key>
			} else if commandTokens[0] == "delete" {
				if len(commandTokens) == 2 {
					var fsReply *string
					var reply *int
					hashedKey := chord.HashString(commandTokens[1])
					chord.Call(net.JoinHostPort(chord.GetLocalAddress(), port), "Server.FindSuccessor", hashedKey, &fsReply)
					chord.Call(*fsReply, "Server.Delete", commandTokens[1], &reply)
					if *reply == 1 {
						chord.PrintPrompt("Delete successful")
					} else {
						chord.PrintPrompt("Key could not be found")
					}
				} else {
					chord.PrintPrompt("Number of arguments supplied incorrect - usage: delete <key>")
				}
				//Display information about the current node - dump
			} else if commandTokens[0] == "dump" {
				var reply *string
				chord.Call(chord.GetLocalAddress()+":"+port, "Server.Dump", sendNothing, &reply)
				fmt.Println(*reply)
				//Display information about a node that owns a specific key - dump <key>
			} else if commandTokens[0] == "dumpkey" {
				if len(commandTokens) == 2 {
					var fsReply, dumpReply *string
					hashedKey := chord.HashString(commandTokens[1])
					chord.Call(net.JoinHostPort(chord.GetLocalAddress(), port), "Server.FindSuccessor", hashedKey, &fsReply)
					chord.Call(*fsReply, "Server.Dump", sendNothing, &dumpReply)
					fmt.Println(*dumpReply)
				} else {
					chord.PrintPrompt("Number of arguments supplied incorrect - usage: dumpkey <key>")
				}
				//Display information about a node with a specific address - dumpaddr <addr>
			} else if commandTokens[0] == "dumpaddr" {
				if len(commandTokens) == 2 {
					var pingReply *int
					var reply *string
					//Make sure the node is alive and listening first
					chord.Call(commandTokens[1], "Server.Ping", sendNothing, &pingReply)
					if pingReply != nil && *pingReply == 562 {
						chord.Call(commandTokens[1], "Server.Dump", sendNothing, &reply)
						chord.PrintPrompt(*reply)
					} else {
						chord.PrintPrompt("Unable to contact server: " + commandTokens[1])
					}
				} else {
					chord.PrintPrompt("Number of arguments supplied incorrect - usage: dumpaddr <addr>")
				}
				//Dump information on all nodes that are part of the ring - dumpall
			} else if commandTokens[0] == "dumpall" {
				var reply *string
				var gsReply []string
				start := chord.GetLocalAddress() + ":" + port
				gsReply = append(gsReply, net.JoinHostPort(chord.GetLocalAddress(), port))
				ringTraversed := false

				for !ringTraversed {
					chord.Call(gsReply[0], "Server.GetSuccessors", sendNothing, &gsReply)
					chord.Call(gsReply[0], "Server.Dump", sendNothing, &reply)
					fmt.Println(*reply)
					if gsReply[0] == start {
						ringTraversed = true
					}
				}
				//List of help commands
			} else if commandTokens[0] == "help" {
				var buffer bytes.Buffer
				buffer.WriteString("\n--- List of Chord Ring Commands --- \n")
				buffer.WriteString("     port <name>       : Set the port the local node should listen on\n")
				buffer.WriteString("     create            : Create a new ring\n")
				buffer.WriteString("     join <address>    : Join an existing ring that has a node with <address> in it\n")
				buffer.WriteString("     quit              : Shutdown the node. If this is the last node in \n")
				buffer.WriteString("                         the ring, the ring also shuts down\n")
				buffer.WriteString("--- Key/Value Operations --- \n")
				buffer.WriteString("     put <key> <value> : Insert the <key> and <value> into the active ring\n")
				buffer.WriteString("     putrandom <n>     : Generates <n> random keys and values and inserts them\n")
				buffer.WriteString("                         into the active ring\n")
				buffer.WriteString("     get <key>         : Find <key> in the active ring\n")
				buffer.WriteString("     delete <key>      : Delete <key> from the active ring\n")
				buffer.WriteString("--- Debugging Commands ---\n")
				buffer.WriteString("     dump              : Display information about the current node\n")
				buffer.WriteString("     dumpkey <key>     : Show information about the node that contains <key>\n")
				buffer.WriteString("     dumpaddr <addr>   : Show information about the node at the given <addr>\n")
				buffer.WriteString("     dumpall           : Show information about all nodes in the active ring\n")
				buffer.WriteString("     ping <addr>       : Check if a node is listening on <addr>\n")
				fmt.Println(buffer.String())
			} else if commandTokens[0] == "quit" {
				chord.PrintPrompt("Shutting down node and transferring keys...")
				var success *int
				var gsReply []string
				var succBucket map[string]string
				address := net.JoinHostPort(chord.GetLocalAddress(), port)
				//Get successor
				chord.Call(address, "Server.GetSuccessors", sendNothing, &gsReply)
				//Get the bucket of this node
				chord.Call(address, "Server.GetAll", sendNothing, &succBucket)
				//Push bucket to successor
				chord.Call(gsReply[0], "Server.PutAll", succBucket, &success)
				//Exit success
				chord.PrintPrompt("all keys transferred now exiting...")
				os.Exit(0)
			} else {
				fmt.Println("Command not recognized")
			}
			chord.PrintPrompt()
		}
	}

	//Error handling
	if err := scanner.Err(); err != nil {
		chord.PrintPrompt()
		fmt.Fprintln(os.Stderr, "Reading standard input:", err)
	}
}