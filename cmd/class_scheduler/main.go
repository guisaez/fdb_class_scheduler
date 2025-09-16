package main

import (
	"fmt"
	"log"
	"strconv"

	"github.com/apple/foundationdb/bindings/go/src/fdb"
	"github.com/apple/foundationdb/bindings/go/src/fdb/directory"
	"github.com/apple/foundationdb/bindings/go/src/fdb/tuple"
)

func main() {
	fdb.MustAPIVersion(730)

	db := fdb.MustOpenDefault()

	// The `directory` module lets us open a `directory` in the database
	// https://apple.github.io/foundationdb/developer-guide.html#developer-guide-directories
	//
	// The CreateOrOpen() function returns a `subspace` where we'll store our application data. Each subspace has a fixed pre-fix
	// it uses when defining keys. The prefix corresponds to the first element of a tuple. We decided that
	// we wanted "attends" and "class" as our prefixes, so we'll create a new subspaces for them within the `scheduling` subspace
	// https://apple.github.io/foundationdb/developer-guide.html#developer-guide-sub-keyspaces
	schedulingDir, err := directory.CreateOrOpen(db, []string{"scheduling"}, nil)
	if err != nil {
		log.Fatal(err)
	}

	courseSS := schedulingDir.Sub("class")
	attendSS := schedulingDir.Sub("attends")

	// Subspaces have a `Pack()` function for defining keys. To store the records for our data model, we can use
	// `attendSS.Pack(tuple.Tuple{studentID, class}) and courseSS.Pack(tuple.Tuple{class})

	// Transactions
	// We are going to rely on the powerful guarantees of transactions to help keep all of our modifications straight, so
	// let's look at how the FoundationDB Go API lets you write a transactional function. We use `Transact()` to
	// execute a code block transactionally. For example, to `signup` a `studentID` for a `class`, we might use:
	_ = func(t fdb.Transactor, studentID, class string) (err error) {
		_, err = t.Transact(func(tr fdb.Transaction) (ret any, err error) {
			tr.Set(attendSS.Pack(tuple.Tuple{studentID, class}), []byte{})
			return
		})
		return
	}

	// A function using this approach take a parameter of type Transactor. When calling such function,
	// you can pass an argument of type Database or Transaction. The function to be executed transactionally
	// is parametrized by the Transaction it wil use to do reads and writes.
	//
	// When using a `Database`, Transact() automatically create a transaction an implements a retry loop to ensure
	// the transaction eventually commits. If you instead pass a Transaction, that transaction will be directly, and
	// it is assumed that the caller implements appropiate retry logic for errors. This permits functions using this pattern
	// to be composed into larger transactions.
	//
	// Withouth the Transact() method, signup would look something like:
	_ = func(db fdb.Database, studentID, class string) (err error) {

		tr, err := db.CreateTransaction()
		if err != nil {
			return
		}

		wrapped := func() {
			defer func() {
				if r := recover(); r != nil {
					e, ok := r.(fdb.Error)
					if ok {
						err = e
					} else {
						panic(r)
					}
				}
			}()

			tr.Set(attendSS.Pack(tuple.Tuple{studentID, class}), []byte{})

			err = tr.Commit().Get()
		}

		for {
			wrapped()

			if err != nil {
				return
			}

			fe, ok := err.(fdb.Error)
			if ok {
				err = tr.OnError(fe).Get()
			}

			if err != nil {
				return
			}
		}
	}

	// Furthermore the version above can only be called with a Database, making it impossible to compose larger transactional
	// functions by calling one from another.
	//
	// Note that by default, teh opration will be retried an infinte number ot times and the transaction will never time out.
	// It is therefore recommended that the client choose a default transaction retry limit or timeout value that is suitable
	// their application. This can be set either at the transaction level using the `SetRetryLimit` or `SetTimeout` transaction
	// options or at the database level with the `SetTransactionRetryLimit` or `SetTransactionTimeout` database options. For example,
	// one can set a one minute timeout on each transaction and a default retry limit of 100 by calling:
	//
	db.Options().SetTransactionTimeout(60000) // 60,000 ms = 1 minute
	db.Options().SetTransactionRetryLimit(100)

	// Making some sample cases
	// Let's make some sample cases and put them in the `classNames` variable. We'll make indivial classes from combinations of class types,
	// levels and times:

	var levels = []string{"intro", "for dummies", "remedial", "101", "201", "301", "mastery", "lab", "seminar"}
	var types = []string{"chem", "bio", "cs", "geometry", "calc", "alg", "film", "music", "art", "dance"}
	var times = []string{"2:00", "3:00", "4:00", "5:00", "6:00", "7:00", "8:00", "9:00", "10:00", "11:00",
		"12:00", "13:00", "14:00", "15:00", "16:00", "17:00", "18:00", "19:00"}

	classes := make([]string, len(levels)*len(types)*len(times))

	for i := range levels {
		for j := range types {
			for k := range times {
				classes[i*len(types)*len(times)+j*len(times)+k] = fmt.Sprintf("%s %s %s", levels[i], types[j], times[k])
			}
		}
	}

	// Next we initialize our database with our class list:
	_, err = db.Transact(func(tr fdb.Transaction) (any, error) {

		tr.ClearRange(schedulingDir)

		for i := range classes {
			tr.Set(courseSS.Pack(tuple.Tuple{classes[i]}), []byte(strconv.FormatInt(100, 10)))
		}

		return nil, nil
	})
	// After this code is run, the database will contain all of the same classes we created above.

	// Listing available classes
	// Before students can do anything else, they need to be able to retrieve a list of available
	// classes from the database. Because FoundationDB sorts its dat by key and therfore hes efficient range-read
	// capability, we can retrive all of the classes in a single database call. We find this range of keys with GetRange():

	_ = func(t fdb.Transactor) (ac []string, err error) {
		r, err := t.ReadTransact(func(rtr fdb.ReadTransaction) (any, error) {
			var classes []string
			ri := rtr.GetRange(courseSS, fdb.RangeOptions{}).Iterator()
			for ri.Advance() {
				kv := ri.MustGet()
				t, err := courseSS.Unpack(kv.Key)
				if err != nil {
					return nil, err
				}
				classes = append(classes, t[0].(string))
			}
			return classes, nil
		})
		if err == nil {
			ac = r.([]string)
		}
		return
	}

	// Signing up for a class
	//
	// We finally get to the curcial function (which we saw before looking at Transact(). A student
	// has decided on a class (by name) and wants to sign up. The signup function will take a student Id and a class:
	//
	_ = func(t fdb.Transactor, studentID, class string) (err error) {
		SCKey := attendSS.Pack(tuple.Tuple{studentID, class})

		_, err = t.Transact(func(tr fdb.Transaction) (ret any, err error) {
			tr.Set(SCKey, []byte{})
			return
		})
		return
	}
	// We simply insert the appropiate record with a blank value.

	// Dropping a class
	//
	_ = func(t fdb.Transactor, studentID, class string) (err error) {
		SCKey := attendSS.Pack(tuple.Tuple{studentID, class})

		_, err = t.Transact(func(tr fdb.Transaction) (ret any, err error) {
			tr.Clear(SCKey)
			return
		})
		return
	}
	
	// Of course, to actually drop the stuent from the calls, we nned to be able to delete the 
	// record from the database. We do this with the Clear() function.
	// 
	
}
