package keys

// Store is the interface required by key manager for data storage.
type Store interface {
	AccountKey(address string) (Storable, error)
	Proposer() (int, error)
	InsertProposer(proposer Proposer) error
	DeleteAllProposers() error
}
