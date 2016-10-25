package notary

// PassRetriever is a callback function that should retrieve a passphrase
// for a given named key. If it should be treated as new passphrase (e.g. with
// confirmation), createNew will be true. Attempts is passed in so that implementers
// decide how many chances to give to a human, for example.
type PassRetriever func(keyName, alias string, createNew bool, attempts int) (passphrase string, giveup bool, err error)
