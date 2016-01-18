// Copyright 2015 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package migration

import (
	"time"

	"github.com/juju/names"

	"github.com/juju/juju/version"
)

type Description interface {
	Model() Model
	// Add/Get binaries
}

type Model interface {
	Tag() names.EnvironTag
	Owner() names.UserTag
	Config() map[string]interface{}
	LatestToolsVersion() version.Number
	Users() []User
	Machines() []Machine

	AddUser(UserArgs)
}

type User interface {
	Name() names.UserTag
	DisplayName() string
	CreatedBy() names.UserTag
	DateCreated() time.Time
	LastConnection() time.Time
	ReadOnly() bool
}

// Address represents an IP Address of some form.
type Address interface {
	Value() string
	Type() string
	NetworkName() string
	Scope() string
	Origin() string
}

// AgentTools represent the version and related binary file
// that the machine and unit agents are using.
type AgentTools interface {
	Version() version.Binary
	URL() string
	SHA256() string
	Size() int64
}

type Machine interface {
	Id() names.MachineTag
	Nonce() string
	PasswordHash() string
	Placement() string
	Instance() CloudInstance
	Series() string
	ContainerType() string
	// Life() string -- only transmit alive things?
	ProviderAddresses() []Address
	MachineAddresses() []Address

	PreferredPublicAddress() Address
	PreferredPrivateAddress() Address

	Tools() AgentTools
	Jobs() []string

	SupportedContainers() ([]string, bool)

	Containers() []Machine

	// TODO:
	// Status, status history
	// Storage
	// Units
}

// CloudInstance holds information particular to a machine
// instance in a cloud.
type CloudInstance interface {
	InstanceId() string
	Status() string
	// The instance attributes below use pointers as they are all
	// optional.
	Architecture() *string
	Memory() *uint64
	RootDisk() *uint64
	CpuCores() *uint64
	CpuPower() *uint64
	Tags() *[]string
	AvailabilityZone() *string
}
