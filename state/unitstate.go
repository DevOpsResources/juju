// Copyright 2020 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package state

import (
	"strconv"

	"github.com/juju/errors"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
	"gopkg.in/mgo.v2/txn"

	mgoutils "github.com/juju/juju/mongo/utils"
)

// unitStateDoc records the state persisted by the charm executing in the unit.
type unitStateDoc struct {
	// DocID is always the same as a unit's global key.
	DocID    string `bson:"_id"`
	TxnRevno int64  `bson:"txn-revno"`

	// The following maps to UnitState:

	// State encodes the unit's persisted state as a list of key-value pairs.
	State map[string]string `bson:"state,omitempty"`

	// UniterState is a serialized yaml string containing the uniters internal
	// state for this unit.
	UniterState string `bson:"uniter-state,omitempty"`

	// RelationState is a serialized yaml string containing relation internal
	// state for this unit from the uniter.
	RelationState map[string]string `bson:"relation-state,omitempty"`

	// StorageState is a serialized yaml string containing storage internal
	// state for this unit from the uniter.
	StorageState string `bson:"storage-state,omitempty"`
}

// stateMatches returns true if the State map within the unitStateDoc matches
// the provided st argument.
func (d *unitStateDoc) stateMatches(st bson.M) bool {
	if len(st) != len(d.State) {
		return false
	}

	for k, v := range d.State {
		if st[k] != v {
			return false
		}
	}

	return true
}

// relationData translate the unitStateDoc's RelationState as
// a map[string]string to a map[int]string, as is needed.
// BSON does not allow ints as a map key.
func (d *unitStateDoc) relationData() (map[int]string, error) {
	if d.RelationState == nil {
		return nil, nil
	}
	// BSON maps cannot have an int as key.
	rState := make(map[int]string, len(d.RelationState))
	for k, v := range d.RelationState {
		kString, err := strconv.Atoi(k)
		if err != nil {
			return nil, err
		}
		rState[kString] = v
	}
	return rState, nil
}

// relationStateMatches returns true if the RelationState map within the
// unitStateDoc matches the provided newRS argument.  Assumes that
// IgnoreRelationsState has been called first, therefore if arg is empty,
// returns a nil map to be set.
func (d *unitStateDoc) relationStateMatches(newRS map[string]string) bool {
	for k, v := range d.RelationState {
		if newRS[k] != v {
			return false
		}
	}
	return true
}

// removeUnitStateOp returns the operation needed to remove the unit state
// document associated with the given globalKey.
func removeUnitStateOp(mb modelBackend, globalKey string) txn.Op {
	return txn.Op{
		C:      unitStatesC,
		Id:     mb.docID(globalKey),
		Remove: true,
	}
}

// UnitState contains the various state saved for this unit,
// including from the charm itself and the uniter.
type UnitState struct {
	// state encodes the unit's persisted state as a list of key-value pairs.
	state    map[string]string
	stateSet bool

	// uniterState is a serialized yaml string containing the uniters internal
	// state for this unit.
	uniterState    string
	uniterStateSet bool

	// relationState is a serialized yaml string containing relation internal
	// state for this unit from the uniter.
	relationState    map[int]string
	relationStateSet bool

	// storageState is a serialized yaml string containing storage internal
	// state for this unit from the uniter.
	storageState    string
	storageStateSet bool
}

// NewUnitState returns a new UnitState struct.
func NewUnitState() *UnitState {
	return &UnitState{}
}

// Modified returns true if any of the struct have been set.
func (u *UnitState) Modified() bool {
	return u.relationStateSet || u.storageStateSet || u.stateSet || u.uniterStateSet
}

// SetState sets the state value.
func (u *UnitState) SetState(state map[string]string) {
	u.stateSet = true
	u.state = state
}

// State returns the unit's state and bool indicating
// whether the data was set.
func (u *UnitState) State() (map[string]string, bool) {
	return u.state, u.stateSet
}

// SetUniterState sets the uniter state value.
func (u *UnitState) SetUniterState(state string) {
	u.uniterStateSet = true
	u.uniterState = state
}

// UniterState returns the uniter state and bool indicating
// whether the data was set.
func (u *UnitState) UniterState() (string, bool) {
	return u.uniterState, u.uniterStateSet
}

// SetRelationState sets the relation state value.
func (u *UnitState) SetRelationState(state map[int]string) {
	u.relationStateSet = true
	u.relationState = state
}

// RelationState returns the relation state and bool indicating
// whether the data was set.
func (u *UnitState) RelationState() (map[int]string, bool) {
	return u.relationState, u.relationStateSet
}

// relationStateBSONFriendly makes a map[int]string BSON friendly by
// translating the int map key to a string.
func (u *UnitState) relationStateBSONFriendly() (map[string]string, bool) {
	stringData := make(map[string]string, len(u.relationState))
	for k, v := range u.relationState {
		stringData[strconv.Itoa(k)] = v
	}
	return stringData, u.relationStateSet
}

// SetStorageState sets the storage state value.
func (u *UnitState) SetStorageState(state string) {
	u.storageStateSet = true
	u.storageState = state
}

// StorageState returns the storage state and bool indicating
// whether the data was set.
func (u *UnitState) StorageState() (string, bool) {
	return u.storageState, u.storageStateSet
}

// SetState replaces the currently stored state for a unit with the contents
// of the provided UnitState.
//
// Use this for testing, otherwise use SetStateOperation.
func (u *Unit) SetState(unitState *UnitState) error {
	modelOp := u.SetStateOperation(unitState)
	return u.st.ApplyOperation(modelOp)
}

// SetStateOperation returns a ModelOperation for replacing the currently
// stored state for a unit with the contents of the provided UnitState.
func (u *Unit) SetStateOperation(unitState *UnitState) ModelOperation {
	return &unitSetStateOperation{u: u, newState: unitState}
}

// State returns the persisted state for a unit.
func (u *Unit) State() (*UnitState, error) {
	us := NewUnitState()
	if u.Life() != Alive {
		return us, errors.NotFoundf("unit %s", u.Name())
	}

	coll, closer := u.st.db().GetCollection(unitStatesC)
	defer closer()

	var stDoc unitStateDoc
	if err := coll.FindId(u.globalKey()).One(&stDoc); err != nil {
		if err == mgo.ErrNotFound {
			return us, nil
		}
		return us, errors.Trace(err)
	}

	if stDoc.RelationState != nil {
		rState, err := stDoc.relationData()
		if err != nil {
			return us, errors.Trace(err)
		}
		us.SetRelationState(rState)
	}

	if stDoc.State != nil {
		unitState := make(map[string]string, len(stDoc.State))
		for k, v := range stDoc.State {
			unitState[mgoutils.UnescapeKey(k)] = v
		}
		us.SetState(unitState)
	}

	us.SetUniterState(stDoc.UniterState)
	us.SetStorageState(stDoc.StorageState)

	return us, nil
}
