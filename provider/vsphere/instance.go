// Copyright 2015 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

// +build !gccgo

package vsphere

import (
	"github.com/juju/errors"
	"github.com/vmware/govmomi/vim25/mo"

	"github.com/juju/juju/instance"
	"github.com/juju/juju/network"
	"github.com/juju/juju/provider/common"
	"github.com/juju/juju/status"
)

type environInstance struct {
	base *mo.VirtualMachine
	env  *environ
}

var _ instance.Instance = (*environInstance)(nil)

func newInstance(base *mo.VirtualMachine, env *environ) *environInstance {
	return &environInstance{
		base: base,
		env:  env,
	}
}

// Id implements instance.Instance.
func (inst *environInstance) Id() instance.Id {
	return instance.Id(inst.base.Name)
}

// Status implements instance.Instance.
func (inst *environInstance) Status() instance.InstanceStatus {
	// TODO(perrito666) I wont change the commented line because it was
	// there and I have not enough knowledge about this provider
	// but that method does not exist.
	// return inst.base.Status()
	return instance.InstanceStatus{
		Status: status.Pending,
	}
}

// Addresses implements instance.Instance.
func (inst *environInstance) Addresses() ([]network.Address, error) {
	if inst.base.Guest == nil || inst.base.Guest.IpAddress == "" {
		return nil, nil
	}
	res := make([]network.Address, 0)
	for _, net := range inst.base.Guest.Net {
		for _, ip := range net.IpAddress {
			res = append(res, network.NewAddress(ip))
		}
	}
	return res, nil
}

func findInst(id instance.Id, instances []instance.Instance) instance.Instance {
	for _, inst := range instances {
		if id == inst.Id() {
			return inst
		}
	}
	return nil
}

// firewall stuff

// OpenPorts opens the given ports on the instance, which
// should have been started with the given machine id.
func (inst *environInstance) OpenPorts(machineID string, rules []network.IngressRule) error {
	return inst.changeIngressRules(true, rules)
}

// ClosePorts closes the given ports on the instance, which
// should have been started with the given machine id.
func (inst *environInstance) ClosePorts(machineID string, rules []network.IngressRule) error {
	return inst.changeIngressRules(false, rules)
}

// IngressRules returns the set of ports open on the instance, which
// should have been started with the given machine id.
func (inst *environInstance) IngressRules(machineID string) ([]network.IngressRule, error) {
	_, client, err := inst.getInstanceConfigurator()
	if err != nil {
		return nil, errors.Trace(err)
	}
	return client.FindIngressRules()
}

func (inst *environInstance) changeIngressRules(insert bool, rules []network.IngressRule) error {
	if inst.env.ecfg.externalNetwork() == "" {
		return errors.New("Can't close/open ports without external network")
	}
	addresses, client, err := inst.getInstanceConfigurator()
	if err != nil {
		return errors.Trace(err)
	}

	for _, addr := range addresses {
		if addr.Scope == network.ScopePublic {
			err = client.ChangeIngressRules(addr.Value, insert, rules)
			if err != nil {
				return errors.Trace(err)
			}
		}
	}
	return nil
}

func (inst *environInstance) getInstanceConfigurator() ([]network.Address, common.InstanceConfigurator, error) {
	addresses, err := inst.Addresses()
	if err != nil {
		return nil, nil, errors.Trace(err)
	}

	var localAddr string
	for _, addr := range addresses {
		if addr.Scope == network.ScopeCloudLocal {
			localAddr = addr.Value
			break
		}
	}

	client := common.NewSshInstanceConfigurator(localAddr)
	return addresses, client, err
}
