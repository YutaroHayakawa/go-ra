// Code generated by deepcopy-gen Config Status InterfaceConfig InterfaceStatus PrefixConfig RouteConfig RDNSSConfig DNSSLConfig; DO NOT EDIT.

package ra

// deepCopy generates a deep copy of *Config
func (o *Config) deepCopy() *Config {
	var cp Config = *o
	if o.Interfaces != nil {
		cp.Interfaces = make([]*InterfaceConfig, len(o.Interfaces))
		copy(cp.Interfaces, o.Interfaces)
		for i2 := range o.Interfaces {
			if o.Interfaces[i2] != nil {
				cp.Interfaces[i2] = o.Interfaces[i2].deepCopy()
			}
		}
	}
	return &cp
}

// deepCopy generates a deep copy of *Status
func (o *Status) deepCopy() *Status {
	var cp Status = *o
	if o.Interfaces != nil {
		cp.Interfaces = make([]*InterfaceStatus, len(o.Interfaces))
		copy(cp.Interfaces, o.Interfaces)
		for i2 := range o.Interfaces {
			if o.Interfaces[i2] != nil {
				cp.Interfaces[i2] = o.Interfaces[i2].deepCopy()
			}
		}
	}
	return &cp
}

// deepCopy generates a deep copy of *InterfaceConfig
func (o *InterfaceConfig) deepCopy() *InterfaceConfig {
	var cp InterfaceConfig = *o
	if o.Prefixes != nil {
		cp.Prefixes = make([]*PrefixConfig, len(o.Prefixes))
		copy(cp.Prefixes, o.Prefixes)
		for i2 := range o.Prefixes {
			if o.Prefixes[i2] != nil {
				cp.Prefixes[i2] = o.Prefixes[i2].deepCopy()
			}
		}
	}
	if o.Routes != nil {
		cp.Routes = make([]*RouteConfig, len(o.Routes))
		copy(cp.Routes, o.Routes)
		for i2 := range o.Routes {
			if o.Routes[i2] != nil {
				cp.Routes[i2] = o.Routes[i2].deepCopy()
			}
		}
	}
	if o.RDNSSes != nil {
		cp.RDNSSes = make([]*RDNSSConfig, len(o.RDNSSes))
		copy(cp.RDNSSes, o.RDNSSes)
		for i2 := range o.RDNSSes {
			if o.RDNSSes[i2] != nil {
				cp.RDNSSes[i2] = o.RDNSSes[i2].deepCopy()
			}
		}
	}
	if o.DNSSLs != nil {
		cp.DNSSLs = make([]*DNSSLConfig, len(o.DNSSLs))
		copy(cp.DNSSLs, o.DNSSLs)
		for i2 := range o.DNSSLs {
			if o.DNSSLs[i2] != nil {
				cp.DNSSLs[i2] = o.DNSSLs[i2].deepCopy()
			}
		}
	}
	return &cp
}

// deepCopy generates a deep copy of *InterfaceStatus
func (o *InterfaceStatus) deepCopy() *InterfaceStatus {
	var cp InterfaceStatus = *o
	return &cp
}

// deepCopy generates a deep copy of *PrefixConfig
func (o *PrefixConfig) deepCopy() *PrefixConfig {
	var cp PrefixConfig = *o
	if o.ValidLifetimeSeconds != nil {
		cp.ValidLifetimeSeconds = new(int)
		*cp.ValidLifetimeSeconds = *o.ValidLifetimeSeconds
	}
	if o.PreferredLifetimeSeconds != nil {
		cp.PreferredLifetimeSeconds = new(int)
		*cp.PreferredLifetimeSeconds = *o.PreferredLifetimeSeconds
	}
	return &cp
}

// deepCopy generates a deep copy of *RouteConfig
func (o *RouteConfig) deepCopy() *RouteConfig {
	var cp RouteConfig = *o
	return &cp
}

// deepCopy generates a deep copy of *RDNSSConfig
func (o *RDNSSConfig) deepCopy() *RDNSSConfig {
	var cp RDNSSConfig = *o
	if o.Addresses != nil {
		cp.Addresses = make([]string, len(o.Addresses))
		copy(cp.Addresses, o.Addresses)
	}
	return &cp
}

// deepCopy generates a deep copy of *DNSSLConfig
func (o *DNSSLConfig) deepCopy() *DNSSLConfig {
	var cp DNSSLConfig = *o
	if o.DomainNames != nil {
		cp.DomainNames = make([]string, len(o.DomainNames))
		copy(cp.DomainNames, o.DomainNames)
	}
	return &cp
}
