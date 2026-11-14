package dep

// ValueFunc is the function that a function value of a value row holds.
func ValueFunc() {}

// ValueGeneric is a generic function that a value row instantiates.
func ValueGeneric[T any]() {}

// ValuePair is a generic function with two type parameters.
func ValuePair[A, B any]() {}

// ValueHook holds a function.
var ValueHook = ValueHooked

// ValueHooked is the function that ValueHook holds.
func ValueHooked() {}

// ValueMethod is the method that a method value of a value row names.
func (t *Thing) ValueMethod() {}
