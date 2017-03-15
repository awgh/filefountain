package main

import (
	"fmt"
	"io"
	"log"
	"math/rand"
	"os"

	"github.com/awgh/bencrypt/bc"
	"github.com/google/gofountain"
)

var (
	//  8192 max sourceblocks for gofountain Raptor implementation
	//  8192 / 8 = 256

	// PKCS7 limits us to 255 byte blocks,
	// since there's no way to tell if something has been padded or not, pad everything
	unpaddedSize = 255
	paddedSize   = unpaddedSize + 1
	wire         = make(chan []fountain.LTBlock, 1)
)

func sendfile(filepath string) {
	file, err := os.Open(filepath) //open input
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	err = nil
	var offset int64
	for err == nil {
		unpadded := make([]byte, unpaddedSize)
		//prng := rand.New(fountain.NewMersenneTwister(seed)) // todo
		n, err := file.ReadAt(unpadded, offset)
		if err == io.EOF {
			unpadded = unpadded[:n]
			if n == 0 {
				break
			}
		} else if err != nil {
			log.Fatal(err)
		}
		offset += int64(n)

		padded, err := bc.Pkcs7Pad(unpadded, paddedSize)
		if err != nil {
			log.Fatal(err)
		}
		codec := fountain.NewRaptorCodec(paddedSize/8, 8) // todo: align 8 for x64, 4 for 32 bit

		ids := make([]int64, paddedSize)
		random := rand.New(rand.NewSource(8923489)) // must match receiver seed
		for i := range ids {
			ids[i] = int64(random.Intn(100000))
		}
		lubyBlocks := fountain.EncodeLTBlocks(padded, ids, codec)

		//log.Println("TX:", len(unpadded), len(padded))
		wire <- lubyBlocks
	}
	wire <- nil
}

func recvfile(outfile *os.File, lubyBlocks []fountain.LTBlock) {

	codec := fountain.NewRaptorCodec(paddedSize/8, 8) // todo: align 8 for x64, 4 for 32 bit
	decoder := codec.NewDecoder(paddedSize)

	determined := decoder.AddBlocks(lubyBlocks)
	if !determined {
		fmt.Printf("After adding code blocks, decoder is still undetermined.")
	}

	decoded := decoder.Decode()
	depadded, err := bc.Pkcs7Unpad(decoded, paddedSize)
	if err != nil {
		log.Fatal(err.Error())
	}
	//log.Println("RX:", len(decoded), len(depadded), int(decoded[len(decoded)-1]))
	outfile.Write(depadded)
}

func main() {

	filepath := "test.mp4"
	outfilepath := "test-output.mp4"

	file, err := os.Open(filepath) //open input
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	outfile, err := os.Create(outfilepath) //create output
	if err != nil {
		log.Fatal(err)
	}
	defer outfile.Close()

	go sendfile(filepath) // stream file to chan

	err = nil

	lubyBlocks := <-wire // read blocks from chan, write to file
	for lubyBlocks != nil {
		recvfile(outfile, lubyBlocks)
		lubyBlocks = <-wire
	}
}
