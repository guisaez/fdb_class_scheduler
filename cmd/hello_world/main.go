package main

import (
	"fmt"
	"log"

	"github.com/apple/foundationdb/bindings/go/src/fdb"
)

func main() {
	// Specify the API version.
	// This allows programs to maintain compatibility even if the API is modified in future versions:
	fdb.MustAPIVersion(730)

	// The API will connect to the FoundationDB cluster indicated by the `default cluster file'
	// https://apple.github.io/foundationdb/administration.html#default-cluster-file
	db := fdb.MustOpenDefault()

	// Let's write a key-value pair
	// When this function returns without error, the modification is durably stored in FoundationDB!
	// Under the covers, this function creates a transaction with a single modification.
	_, err := db.Transact(func(tr fdb.Transaction) (ret interface{}, e error) {
		tr.Set(fdb.Key("hello"), []byte("world"))
		return
	})
	if err != nil {
		log.Fatalf("Unable to set FDB database value (%v)", err)
	}

	// Let's read back the data.
	ret, err := db.Transact(func(tr fdb.Transaction) (ret interface{}, e error) {
		// MustGet ensures a value is returned (nil) if not found.
		ret = tr.Get(fdb.Key("hello")).MustGet()
		return
	})
	if err != nil {
		log.Fatalf("Unable to read FDB database value (%v)", err)
	}

	v := ret.([]byte)

	fmt.Printf("hello, %s\n", string(v))
}
