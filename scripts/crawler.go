package main

import (
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"time"
)

type Address [16]byte

// GetPrefix returns the address prefix used by yggdrasil.
// The current implementation requires this to be a multiple of 8 bits + 7 bits.
// The 8th bit of the last byte is used to signal nodes (0) or /64 prefixes (1).
// Nodes that configure this differently will be unable to communicate with each other using IP packets, though routing and the DHT machinery *should* still work.
func GetPrefix() [1]byte {
	return [...]byte{0x02}
}

// AddrForKey takes an ed25519.PublicKey as an argument and returns an *Address.
// This function returns nil if the key length is not ed25519.PublicKeySize.
// This address begins with the contents of GetPrefix(), with the last bit set to 0 to indicate an address.
// The following 8 bits are set to the number of leading 1 bits in the bitwise inverse of the public key.
// The bitwise inverse of the key, excluding the leading 1 bits and the first leading 0 bit, is truncated to the appropriate length and makes up the remainder of the address.
func AddrForKey(publicKey ed25519.PublicKey) *Address {
	// 128 bit address
	// Begins with prefix
	// Next bit is a 0
	// Next 7 bits, interpreted as a uint, are # of leading 1s in the NodeID
	// Leading 1s and first leading 0 of the NodeID are truncated off
	// The rest is appended to the IPv6 address (truncated to 128 bits total)
	if len(publicKey) != ed25519.PublicKeySize {
		return nil
	}
	var buf [ed25519.PublicKeySize]byte
	copy(buf[:], publicKey)
	for idx := range buf {
		buf[idx] = ^buf[idx]
	}
	var addr Address
	var temp = make([]byte, 0, 32)
	done := false
	ones := byte(0)
	bits := byte(0)
	nBits := 0
	for idx := 0; idx < 8*len(buf); idx++ {
		bit := (buf[idx/8] & (0x80 >> byte(idx%8))) >> byte(7-(idx%8))
		if !done && bit != 0 {
			ones++
			continue
		}
		if !done && bit == 0 {
			done = true
			continue // FIXME? this assumes that ones <= 127, probably only worth changing by using a variable length uint64, but that would require changes to the addressing scheme, and I'm not sure ones > 127 is realistic
		}
		bits = (bits << 1) | bit
		nBits++
		if nBits == 8 {
			nBits = 0
			temp = append(temp, bits)
		}
	}
	prefix := GetPrefix()
	copy(addr[:], prefix[:])
	addr[len(prefix)] = ones
	copy(addr[len(prefix)+1:], temp)
	return &addr
}

func addrToAddr(inputAddr string) string {
	decoded, err := hex.DecodeString(inputAddr)
	if err != nil {
		panic(err)
	}
	publicKey := ed25519.PublicKey(decoded)
	addr := net.IP(AddrForKey(publicKey)[:])
	return addr.String()
}

var waitgroup sync.WaitGroup
var visited sync.Map
var rumored sync.Map

const MAX_RETRY = 3
const N_PARALLEL_REQ = 32

var semaphore chan struct{}

func init() {
  semaphore = make(chan struct{}, N_PARALLEL_REQ)
}

func dial() (net.Conn, error) {
	return net.DialTimeout("unix", "/var/run/yggdrasil.sock", time.Second)
}

func getRequest(key, request string) map[string]interface{} {
	arguments := map[string]interface{}{
		"key":       key,
	}
	return map[string]interface{}{
		"keepalive": true,
		"request":   request,
		"arguments": arguments,
	}
}

func doRequest(request map[string]interface{}) map[string]interface{} {
	req, err := json.Marshal(request)
	if err != nil {
		panic(err)
	}
	var res map[string]interface{}
	for idx := 0; idx < MAX_RETRY; idx++ {
		sock, err := dial()
		if err != nil {
			panic(err)
		}
		if _, err = sock.Write(req); err != nil {
			panic(err)
		}
		bs := make([]byte, 65535)
		n, err := sock.Read(bs)
		if err != nil {
			panic(bs)
		}
		bs = bs[:n]
		if err = json.Unmarshal(bs, &res); err != nil {
			panic(err)
		}
		// TODO parse res, check if there's an error
		if res, ok := res["response"]; ok {
			if res == nil {
				continue
			}
			if _, isIn := res.(map[string]interface{})["error"]; isIn {
				continue
			}
		}
		break
	}
	return res
}

func getNodeInfo(key string) map[string]interface{} {
	return doRequest(getRequest(key, "getNodeInfo"))
}

func getSelf(key string) map[string]interface{} {
	return doRequest(getRequest(key, "debug_remoteGetSelf"))
}

func getPeers(key string) map[string]interface{} {
	return doRequest(getRequest(key, "debug_remoteGetPeers"))
}

func getDHT(key string) map[string]interface{} {
	return doRequest(getRequest(key, "debug_remoteGetDHT"))
}

type rumorResult struct {
	key string
	res map[string]interface{}
}

func doRumor(key string, out chan rumorResult) {
	waitgroup.Add(1)
	go func() {
		defer waitgroup.Done()
		semaphore<-struct{}{}
		defer func() { <-semaphore }()
		if _, known := rumored.LoadOrStore(key, true); known {
			return
		}
		defer rumored.Delete(key)
		if _, known := visited.Load(key); known {
			return
		}
		results := make(map[string]interface{})
		if res, ok := getNodeInfo(key)["response"]; ok {
			if res == nil {
				return
			}
			for addr, v := range res.(map[string]interface{}) {
				vm, ok := v.(map[string]interface{})
				if !ok {
					return
				}
				results["address"] = addr
				results["nodeinfo"] = vm
			}
		}
		if res, ok := getSelf(key)["response"]; ok {
			for _, v := range res.(map[string]interface{}) {
				vm, ok := v.(map[string]interface{})
				if !ok {
					return
				}
				if coords, ok := vm["coords"]; ok {
					results["coords"] = coords
				}
			}
		}
		if res, ok := getPeers(key)["response"]; ok {
			for _, v := range res.(map[string]interface{}) {
				vm, ok := v.(map[string]interface{})
				if !ok {
					return
				}
				if keys, ok := vm["keys"]; ok {
					results["peers"] = keys
					for _, key := range keys.([]interface{}) {
						doRumor(key.(string), out)
					}
				}
			}
		}
		if res, ok := getDHT(key)["response"]; ok {
			for _, v := range res.(map[string]interface{}) {
				vm, ok := v.(map[string]interface{})
				if !ok {
					return
				}
				if keys, ok := vm["keys"]; ok {
					results["dht"] = keys
					for _, key := range keys.([]interface{}) {
						doRumor(key.(string), out)
					}
				}
			}
		}
		if len(results) > 0 {
			if _, known := visited.LoadOrStore(key, true); known {
				return
			}
			results["time"] = time.Now().Unix()
			out <- rumorResult{key, results}
		}
	}()
}

func doPrinter() (chan rumorResult, chan struct{}) {
	results := make(chan rumorResult)
	done := make(chan struct{})
	go func() {
		defer close(done)
		fmt.Println("{\"yggnodes\": {")
		var notFirst bool
		for result := range results {
			// TODO correct output
			res, err := json.Marshal(result.res)
			if err != nil {
				panic(err)
			}
			if notFirst {
				fmt.Println(",")
			}
			fmt.Printf("\"%s\": %s", result.key, res)
			notFirst = true
		}
		fmt.Println("\n}}")
	}()
	return results, done
}

func main() {
	self := doRequest(map[string]interface{}{"keepalive": true, "request": "getSelf"})
	res := self["response"].(map[string]interface{})
	var key string = res["key"].(string)
	results, done := doPrinter()
	doRumor(key, results)
	waitgroup.Wait()
	close(results)
	<-done
}
