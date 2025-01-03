package python_dependency_usage

// UsageModuleItem represents each item being imported within a module
type IdentifierItem struct {
	Identifier string
	Alias      string
	ItemName   string
}

// ImportModule represents the module import details
type ImportModule struct {
	Used        bool
	Identifiers []IdentifierItem
}

type UsageDiagnostics struct {
	moduleUsageStatus    map[string]ImportModule
	moduleIdentifierKeys map[string]string
}

// Set the module corresponding to this identifier as used
func (d *UsageDiagnostics) setUsed(identifier string) bool {
	targetModuleKey, exists := d.moduleIdentifierKeys[identifier]
	if !exists {
		return false
	}
	module := d.moduleUsageStatus[targetModuleKey]
	module.Used = true
	d.moduleUsageStatus[targetModuleKey] = module
	return true
}

// Check if the module is used
func (d *UsageDiagnostics) isUsed(identifier string) bool {
	targetModuleKey, exists := d.moduleIdentifierKeys[identifier]
	if !exists {
		return false
	}
	module := d.moduleUsageStatus[targetModuleKey]
	return module.Used
}
