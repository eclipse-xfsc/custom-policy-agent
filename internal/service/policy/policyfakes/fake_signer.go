// Code generated by counterfeiter. DO NOT EDIT.
package policyfakes

import (
	"context"
	"sync"

	"github.com/eclipse-xfsc/custom-policy-agent/internal/service/policy"
)

type FakeSigner struct {
	KeyStub        func(context.Context, string, string) (any, error)
	keyMutex       sync.RWMutex
	keyArgsForCall []struct {
		arg1 context.Context
		arg2 string
		arg3 string
	}
	keyReturns struct {
		result1 any
		result2 error
	}
	keyReturnsOnCall map[int]struct {
		result1 any
		result2 error
	}
	SignStub        func(context.Context, string, string, []byte) ([]byte, error)
	signMutex       sync.RWMutex
	signArgsForCall []struct {
		arg1 context.Context
		arg2 string
		arg3 string
		arg4 []byte
	}
	signReturns struct {
		result1 []byte
		result2 error
	}
	signReturnsOnCall map[int]struct {
		result1 []byte
		result2 error
	}
	invocations      map[string][][]interface{}
	invocationsMutex sync.RWMutex
}

func (fake *FakeSigner) Key(arg1 context.Context, arg2 string, arg3 string) (any, error) {
	fake.keyMutex.Lock()
	ret, specificReturn := fake.keyReturnsOnCall[len(fake.keyArgsForCall)]
	fake.keyArgsForCall = append(fake.keyArgsForCall, struct {
		arg1 context.Context
		arg2 string
		arg3 string
	}{arg1, arg2, arg3})
	stub := fake.KeyStub
	fakeReturns := fake.keyReturns
	fake.recordInvocation("Key", []interface{}{arg1, arg2, arg3})
	fake.keyMutex.Unlock()
	if stub != nil {
		return stub(arg1, arg2, arg3)
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	return fakeReturns.result1, fakeReturns.result2
}

func (fake *FakeSigner) KeyCallCount() int {
	fake.keyMutex.RLock()
	defer fake.keyMutex.RUnlock()
	return len(fake.keyArgsForCall)
}

func (fake *FakeSigner) KeyCalls(stub func(context.Context, string, string) (any, error)) {
	fake.keyMutex.Lock()
	defer fake.keyMutex.Unlock()
	fake.KeyStub = stub
}

func (fake *FakeSigner) KeyArgsForCall(i int) (context.Context, string, string) {
	fake.keyMutex.RLock()
	defer fake.keyMutex.RUnlock()
	argsForCall := fake.keyArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2, argsForCall.arg3
}

func (fake *FakeSigner) KeyReturns(result1 any, result2 error) {
	fake.keyMutex.Lock()
	defer fake.keyMutex.Unlock()
	fake.KeyStub = nil
	fake.keyReturns = struct {
		result1 any
		result2 error
	}{result1, result2}
}

func (fake *FakeSigner) KeyReturnsOnCall(i int, result1 any, result2 error) {
	fake.keyMutex.Lock()
	defer fake.keyMutex.Unlock()
	fake.KeyStub = nil
	if fake.keyReturnsOnCall == nil {
		fake.keyReturnsOnCall = make(map[int]struct {
			result1 any
			result2 error
		})
	}
	fake.keyReturnsOnCall[i] = struct {
		result1 any
		result2 error
	}{result1, result2}
}

func (fake *FakeSigner) Sign(arg1 context.Context, arg2 string, arg3 string, arg4 []byte) ([]byte, error) {
	var arg4Copy []byte
	if arg4 != nil {
		arg4Copy = make([]byte, len(arg4))
		copy(arg4Copy, arg4)
	}
	fake.signMutex.Lock()
	ret, specificReturn := fake.signReturnsOnCall[len(fake.signArgsForCall)]
	fake.signArgsForCall = append(fake.signArgsForCall, struct {
		arg1 context.Context
		arg2 string
		arg3 string
		arg4 []byte
	}{arg1, arg2, arg3, arg4Copy})
	stub := fake.SignStub
	fakeReturns := fake.signReturns
	fake.recordInvocation("Sign", []interface{}{arg1, arg2, arg3, arg4Copy})
	fake.signMutex.Unlock()
	if stub != nil {
		return stub(arg1, arg2, arg3, arg4)
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	return fakeReturns.result1, fakeReturns.result2
}

func (fake *FakeSigner) SignCallCount() int {
	fake.signMutex.RLock()
	defer fake.signMutex.RUnlock()
	return len(fake.signArgsForCall)
}

func (fake *FakeSigner) SignCalls(stub func(context.Context, string, string, []byte) ([]byte, error)) {
	fake.signMutex.Lock()
	defer fake.signMutex.Unlock()
	fake.SignStub = stub
}

func (fake *FakeSigner) SignArgsForCall(i int) (context.Context, string, string, []byte) {
	fake.signMutex.RLock()
	defer fake.signMutex.RUnlock()
	argsForCall := fake.signArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2, argsForCall.arg3, argsForCall.arg4
}

func (fake *FakeSigner) SignReturns(result1 []byte, result2 error) {
	fake.signMutex.Lock()
	defer fake.signMutex.Unlock()
	fake.SignStub = nil
	fake.signReturns = struct {
		result1 []byte
		result2 error
	}{result1, result2}
}

func (fake *FakeSigner) SignReturnsOnCall(i int, result1 []byte, result2 error) {
	fake.signMutex.Lock()
	defer fake.signMutex.Unlock()
	fake.SignStub = nil
	if fake.signReturnsOnCall == nil {
		fake.signReturnsOnCall = make(map[int]struct {
			result1 []byte
			result2 error
		})
	}
	fake.signReturnsOnCall[i] = struct {
		result1 []byte
		result2 error
	}{result1, result2}
}

func (fake *FakeSigner) Invocations() map[string][][]interface{} {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	fake.keyMutex.RLock()
	defer fake.keyMutex.RUnlock()
	fake.signMutex.RLock()
	defer fake.signMutex.RUnlock()
	copiedInvocations := map[string][][]interface{}{}
	for key, value := range fake.invocations {
		copiedInvocations[key] = value
	}
	return copiedInvocations
}

func (fake *FakeSigner) recordInvocation(key string, args []interface{}) {
	fake.invocationsMutex.Lock()
	defer fake.invocationsMutex.Unlock()
	if fake.invocations == nil {
		fake.invocations = map[string][][]interface{}{}
	}
	if fake.invocations[key] == nil {
		fake.invocations[key] = [][]interface{}{}
	}
	fake.invocations[key] = append(fake.invocations[key], args)
}

var _ policy.Signer = new(FakeSigner)
