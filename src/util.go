package src

import (
	"crypto/sha1"
	"net"
	"net/rpc"
	"fmt"
	"log"
	"math/big"
	"math/rand"
)

const keySize = sha1.Size * 8

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

// HashString Sha-1 hashes a string
func HashString(elt string) *big.Int {
	hasher := sha1.New()
	hasher.Write([]byte(elt))
	return new(big.Int).SetBytes(hasher.Sum(nil))
}

// Returns true if 'elt' is between 'start' and 'end' in chord ring, false otherwise
//if 'inclusive' is true then 'end' considered in between
func Between(start, elt, end *big.Int, inclusive bool) bool {
	if end.Cmp(start) > 0 {
		return (start.Cmp(elt) < 0 && elt.Cmp(end) < 0) || (inclusive && elt.Cmp(end) == 0)
	} else {
		return start.Cmp(elt) < 0 || elt.Cmp(end) < 0 || (inclusive && elt.Cmp(end) == 0)
	}
}

//Get IP address
func GetLocalAddress() string {
	var localaddress string
	ifaces, err := net.Interfaces()
	if err != nil {
		panic("init: failed to find network interfaces")
	}
	//Find the first non-loopback interface with an IP address
	for _, elt := range ifaces {
		if elt.Flags&net.FlagLoopback == 0 && elt.Flags&net.FlagUp != 0 {
			addrs, err := elt.Addrs()
			if err != nil {
				panic("init: failed to get addresses for network interface")
			}
			for _, addr := range addrs {
				if ipnet, ok := addr.(*net.IPNet); ok {
					if ip4 := ipnet.IP.To4(); len(ip4) == net.IPv4len {
						localaddress = ip4.String()
						break
					}
				}
			}
		}
	}
	if localaddress == "" {
		panic("init: failed to find non-loopback interface with valid address on this node")
	}
	return localaddress
}

//Make an RPC call at 'address' with name 'method' and load results of call into 'reply'
func Call(address string, method string, request interface{}, reply interface{}) error {
	client, err := rpc.DialHTTP("tcp", address)
	if err != nil {
		return err
	}

	if err = client.Call(method, request, reply); err != nil {
		log.Fatalf("Error calling "+method+": %v", err)
	}
	client.Close()
	return err
}

//Create a random string of length n that can include special and uppercase letters

func RandString(n int, special bool, uc bool) string {
	var ucletters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
	var lcletters = []rune("abcdefghijklmnopqrstuvwxyz")
	var ucchars = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ!@#$%^&*.-|")
	var lcchars = []rune("abcdefghijklmnopqrstuvwxyz!@#$%^&*.-|")

	b := make([]rune, n)
	if special && uc {
		for i := range b {
			b[i] = ucchars[rand.Intn(len(ucchars))]
		}
	} else if !special && uc {
		for i := range b {
			b[i] = ucletters[rand.Intn(len(ucletters))]
		}
	} else if special && !uc {
		for i := range b {
			b[i] = lcchars[rand.Intn(len(lcchars))]
		}
	} else {
		for i := range b {
			b[i] = lcletters[rand.Intn(len(lcletters))]
		}
	}
	return string(b)
}

func PrintPrompt(args ...string) {
	if len(args) == 0 {
		fmt.Printf("chord> ")
	} else if len(args) == 1 {
		fmt.Printf("chord> " + args[0] + "\n")
	} else {
		fmt.Printf("chord> "+args[0]+"\n", args[1:])
	}
}