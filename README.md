# Class Scheduling in Go

This tutorial provies a walkthrough of designing and building a simple application in Go using FoundationDB. In this tutorial, we use a few simple data modeling techniques. For a more in-depth discussion of data modeling in FodunationDB see [Data Modeling](https://apple.github.io/foundationdb/data-modeling.html)

## Class scheduling application
Let's say we've been asked to build a class scheduling system for students and administrators.

### Requirments
We will need to let users list available classes and track which students have signed up for which clases. Here's a first cut at the functions we'll need to implement:

```
availableClases() // returns list of classes
signup()          // signs up a student for a class
drop()            // drops a student from a class
```

### Data model
First, we need to design a [data mode](https://apple.github.io/foundationdb/data-modeling.html).
A data model is just a method for storing our application data using keys and values in FoundationDB. We seem to have two main types of data: (1) a list of classes and (2) a record of which students will attend which classes. 

Let's keep attending data like this:
```
// ("attends", student, class) = ""
```

We'll just store the key with a blank value to indicate that a student is signed up for a particular class. For this application, we're going to think about a key-value pair's key as a [tuple](https://apple.github.io/foundationdb/data-modeling.html#data-modeling-tuples). Encoding a tuple of data elements into a key is a very common pattern for an orderd key-value store.

We'll keep data about classes like this:
```
// ("class", class_name) = seatsAvailable
```

Similarly, each such key will represent an available class. We’ll use `seatsAvailable` to record the number of seats available.

### Directories and Subspaces

FoundationDB includes a few modules that make it easy to model data using this approach:

```go
import (
  "github.com/apple/foundationdb/bindings/go/src/fdb"
  "github.com/apple/foundationdb/bindings/go/src/fdb/directory"
  "github.com/apple/foundationdb/bindings/go/src/fdb/subspace"
  "github.com/apple/foundationdb/bindings/go/src/fdb/tuple"
)
```


