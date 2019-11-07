// Copyright 2019 the orbs-network-go authors
// This file is part of the orbs-network-go library in the Orbs project.
//
// This source code is licensed under the MIT license found in the LICENSE file in the root directory of this source tree.
// The above notice should be included in all copies or substantial portions of the software.

package sdk

import (
	"context"
	"fmt"
	sdkContext "github.com/orbs-network/orbs-contract-sdk/go/context"
	"github.com/orbs-network/orbs-network-go/services/processor/native/call"
	"github.com/orbs-network/orbs-network-go/services/processor/native/types"
	"github.com/orbs-network/orbs-spec/types/go/primitives"
	"github.com/orbs-network/orbs-spec/types/go/protocol"
	"github.com/orbs-network/orbs-spec/types/go/services/handlers"
	"github.com/pkg/errors"
)

const SDK_OPERATION_NAME_EVENTS = "Sdk.Events"

func (s *service) SdkEventsEmitEvent(executionContextId sdkContext.ContextId, permissionScope sdkContext.PermissionScope, eventFunctionSignature interface{}, args ...interface{}) {
	eventName, err := types.GetContractMethodNameFromFunction(eventFunctionSignature)
	if err != nil {
		panic(err.Error())
	}

	// verify event arguments are allowed to be packed and match signature
	functionNameForErrors := fmt.Sprintf("EVENTS.%s", eventName)
	eventArguments, err := protocol.ArgumentArrayFromNatives(args)
	if err != nil {
		panic(errors.Wrap(err, "event input arguments"))
	}
	_, err = call.VerifyMethodInputArgs(eventFunctionSignature, functionNameForErrors, args)
	if err != nil {
		panic(errors.Wrap(err, "incorrect types given to event emit"))
	}

	_, err = s.sdkHandler.HandleSdkCall(context.TODO(), &handlers.HandleSdkCallInput{
		ContextId:     primitives.ExecutionContextId(executionContextId),
		OperationName: SDK_OPERATION_NAME_EVENTS,
		MethodName:    "emitEvent",
		InputArguments: []*protocol.Argument{
			(&protocol.ArgumentBuilder{
				// eventName
				Type:        protocol.ARGUMENT_TYPE_STRING_VALUE,
				StringValue: eventName,
			}).Build(),
			(&protocol.ArgumentBuilder{
				// inputArgs
				Type:       protocol.ARGUMENT_TYPE_BYTES_VALUE,
				BytesValue: eventArguments.Raw(),
			}).Build(),
		},
		PermissionScope: protocol.ExecutionPermissionScope(permissionScope),
	})
	if err != nil {
		panic(err.Error())
	}
}
