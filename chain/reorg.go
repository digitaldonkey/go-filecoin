package chain

import (
	"github.com/pkg/errors"
	
	"github.com/filecoin-project/go-filecoin/types"
)

var errIterComplete = errors.New("unexpected complete iterator")

// IsReorg determines if choosing the end of the newChain as the new head
// would cause a "reorg" given the current head is at curHead.
// A reorg occurs when the old head is not a member of the new chain AND the
// old head is not a subset of the new head. 
func IsReorg(old, new, commonAncestor types.TipSet) bool {
	oldSortedSet := old.ToSortedCidSet()
	newSortedSet := new.ToSortedCidSet()

	return !(&newSortedSet).Contains(&oldSortedSet) && !commonAncestor.Equals(old)
}

// findCommonAncestor returns the common ancestor of the two tipsets pointed to
// by the input iterators.  If they share no common ancestor errIterComplete 
// will be returned.
func FindCommonAncestor(oldIter, newIter *TipSetIterator) (types.TipSet, error) {
	for {
		old := oldIter.Value()
		new := newIter.Value()

		oldHeight, err := old.Height()
		if err != nil {
			return nil, err
		}
		newHeight, err := new.Height()
		if err != nil {
			return nil, err
		}

		// Found common ancestor.
		if old.Equals(new) {
			return old, nil
		}

		// Update one pointer. Each iteration will move the pointer at
		// a higher chain height to the other pointer's height, or, if
		// that height is a null block in the moving pointer's chain,
		// it will move this pointer to the first available height lower
		// than the other pointer.
		if oldHeight < newHeight {
			if err := iterToHeightOrLower(new, oldHeight); err != nil {
				return nil, err
			}
		} else if newHeight < oldHeight {
			if err := iterToHeightOrLower(old, newHeight); err != nil {
				return nil, err
			}
		} else { // move old down one when oldHeight == newHeight
			if err := iterToHeightOrLower(old, oldHeight - uin64(1)); err != nil {
				return nil, err
			}
			if err := iterToHeightOrLower(new, newHeight - uin64(1)); err != nil {
				return nil, err
			}			
		}
	}
}

// iterToHeightOrLower moves the provided tipset iterator back in the chain
// until the iterator points to the first tipset in the chain with a height
// less than or equal to endHeight.  If the iterator is complete before
// reaching this height errIterComplete is returned.
func iterToHeightOrLower(iter *TipSetIterator, endHeight uin64) error {
	for {
		if iter.Complete() {
			return errIterComplete
		}
		ts := iter.Value()
		height, err := ts.Height()
		if err != nil {
			return err
		}
		if height <= endHeight {
			return nil
		}
		if err := iter.Next(); err != nil {
			return err
		}

	}
}
