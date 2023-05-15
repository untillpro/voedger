/*
 * Copyright (c) 2021-present Sigma-Soft, Ltd.
 * @author: Nikolay Nikitin
 */

package appdef

import (
	"fmt"
	"strings"
)

const SystemContainer_ViewPartitionKey = SystemPackagePrefix + "pkey"
const SystemContainer_ViewClusteringCols = SystemPackagePrefix + "ccols"
const SystemContainer_ViewKey = SystemPackagePrefix + "key"
const SystemContainer_ViewValue = SystemPackagePrefix + "val"

// # Implements:
//   - Container
type container struct {
	name      string
	def       QName
	minOccurs Occurs
	maxOccurs Occurs
}

func newContainer(name string, def QName, minOccurs, maxOccurs Occurs) container {
	return container{
		name:      name,
		def:       def,
		minOccurs: minOccurs,
		maxOccurs: maxOccurs,
	}
}

func (cont *container) Def() QName { return cont.def }

func (cont *container) IsSys() bool { return IsSysContainer(cont.name) }

func (cont *container) MaxOccurs() Occurs { return cont.maxOccurs }

func (cont *container) MinOccurs() Occurs { return cont.minOccurs }

func (cont *container) Name() string { return cont.name }

// Returns is container system
func IsSysContainer(n string) bool {
	return strings.HasPrefix(n, SystemPackagePrefix) && // fast check
		// then more accuracy
		((n == SystemContainer_ViewPartitionKey) ||
			(n == SystemContainer_ViewClusteringCols) ||
			(n == SystemContainer_ViewKey) ||
			(n == SystemContainer_ViewValue))
}

// # Implements:
//   - IWithContainers
//   - IContainersBuilder
type containers struct {
	def               *def
	containers        map[string]*container
	containersOrdered []string
}

func makeContainers(def *def) containers {
	c := containers{def, make(map[string]*container), make([]string, 0)}
	return c
}

func (c *containers) AddContainer(name string, contDef QName, minOccurs, maxOccurs Occurs) IContainersBuilder {
	if name == NullName {
		panic(fmt.Errorf("%v: empty container name: %w", c.def.QName(), ErrNameMissed))
	}
	if !IsSysContainer(name) {
		if ok, err := ValidIdent(name); !ok {
			panic(fmt.Errorf("%v: invalid container name «%v»: %w", c.def.QName(), name, err))
		}
	}
	if c.Container(name) != nil {
		panic(fmt.Errorf("%v: container «%v» is already exists: %w", c.def.QName(), name, ErrNameUniqueViolation))
	}

	if maxOccurs == 0 {
		panic(fmt.Errorf("%v: max occurs value (0) must be positive number: %w", c.def.QName(), ErrInvalidOccurs))
	}
	if maxOccurs < minOccurs {
		panic(fmt.Errorf("%v: max occurs (%v) must be greater or equal to min occurs (%v): %w", c.def.QName(), maxOccurs, minOccurs, ErrInvalidOccurs))
	}

	if !c.def.Kind().ContainersAllowed() {
		panic(fmt.Errorf("%v: definition kind «%v» does not allow containers: %w", c.def.QName(), c.def.Kind(), ErrInvalidDefKind))
	}
	if cd := c.def.App().DefByName(contDef); cd != nil {
		if !c.def.Kind().ContainerKindAvailable(cd.Kind()) {
			panic(fmt.Errorf("%v: definition kind «%v» does not support child container kind «%v»: %w", c.def.QName(), c.def.Kind(), cd.Kind(), ErrInvalidDefKind))
		}
	}

	if len(c.containers) >= MaxDefContainerCount {
		panic(fmt.Errorf("%v: maximum container count (%d) exceeds: %w", c.def.QName(), MaxDefContainerCount, ErrTooManyContainers))
	}

	cont := newContainer(name, contDef, minOccurs, maxOccurs)
	c.containers[name] = &cont
	c.containersOrdered = append(c.containersOrdered, name)

	c.def.changed()

	return c
}

func (c *containers) Container(name string) IContainer {
	if c, ok := c.containers[name]; ok {
		return c
	}
	return nil
}

func (c *containers) ContainerCount() int {
	return len(c.containersOrdered)
}

func (c *containers) ContainerDef(contName string) IDef {
	if cont := c.Container(contName); cont != nil {
		return c.def.App().Def(cont.Def())
	}
	return NullDef
}

func (c *containers) Containers(cb func(IContainer)) {
	for _, n := range c.containersOrdered {
		cb(c.Container(n))
	}
}
