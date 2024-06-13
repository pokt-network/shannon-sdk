package sdk

import (
	"context"
	"encoding/hex"
	"slices"

	ring_secp256k1 "github.com/athanorlabs/go-dleq/secp256k1"
	ringtypes "github.com/athanorlabs/go-dleq/types"
	"github.com/noot/ring-go"
	"github.com/pokt-network/poktroll/pkg/crypto/rings"
	apptypes "github.com/pokt-network/poktroll/x/application/types"
	servicetypes "github.com/pokt-network/poktroll/x/service/types"

	"github.com/pokt-network/shannon-sdk/types"
)

// ApplicationLister returns all the applications or a single application with the specified address.
//
//	It is used by the SDK to perform functions related to applications, e.g. listing applications delegating to a gateway address.
//
// DISCUSS: it may be possible to remove the need for this through providing helper functions and/or methods on the application struct.
type ApplicationLister interface {
	GetAllApplications(context.Context) ([]apptypes.Application, error)
	GetApplication(ctx context.Context, appAddress string) (apptypes.Application, error)
}

// ShannonSDK is the main struct for the SDK that will be used by the service
// to interact with the Shannon network
// TODO_TEST: Add unit tests for the ShannonSDK struct
type ShannonSDK struct {
	ApplicationLister
	sessionClient SessionClient
	accountClient AccountClient
	paramsClient  SharedParamsClient
	blockClient   BlockClient
	relayClient   RelayClient
	signer        Signer
}

// NewShannonSDK creates a new ShannonSDK instance with the given clients and signer.
// The clients are used to interact with the Shannon network.
// The signer is used to sign the relay requests.
func NewShannonSDK(
	applicationLister ApplicationLister,
	sessionClient SessionClient,
	accountClient AccountClient,
	paramsClient SharedParamsClient,
	blockClient BlockClient,
	relayClient RelayClient,
	signer Signer,
) (*ShannonSDK, error) {
	return &ShannonSDK{
		ApplicationLister: applicationLister,
		sessionClient:     sessionClient,
		accountClient:     accountClient,
		paramsClient:      paramsClient,
		blockClient:       blockClient,
		relayClient:       relayClient,
		signer:            signer,
	}, nil
}

// GetSessionSupplierEndpoints returns the current session with its assigned
// suppliers and their corresponding endpoints for the given application address
// and service id.
func (sdk *ShannonSDK) GetSessionSupplierEndpoints(
	ctx context.Context,
	appAddress string,
	serviceId string,
) (sessionSuppliers *types.SessionSuppliers, err error) {
	latestHeight, err := sdk.blockClient.GetLatestBlockHeight(ctx)
	if err != nil {
		return nil, err
	}

	currentSession, err := sdk.sessionClient.GetSession(ctx, appAddress, serviceId, latestHeight)
	if err != nil {
		return nil, err
	}

	sessionSuppliers = &types.SessionSuppliers{
		Session:            currentSession,
		SuppliersEndpoints: make([]*types.SingleSupplierEndpoint, 0),
	}

	for _, supplier := range currentSession.Suppliers {
		for _, service := range supplier.Services {
			if service.Service.Id != serviceId {
				continue
			}

			for _, endpoint := range service.Endpoints {
				sessionSuppliers.SuppliersEndpoints = append(
					sessionSuppliers.SuppliersEndpoints,
					&types.SingleSupplierEndpoint{
						RpcType:         endpoint.RpcType,
						Url:             endpoint.Url,
						SupplierAddress: supplier.Address,
						SessionHeader:   currentSession.Header,
					},
				)
			}
		}
	}

	return sessionSuppliers, nil
}

// GetApplicationsDelegatingToGateway returns the application addresses that are
// delegating to the given gateway address.
func (sdk *ShannonSDK) GetApplicationsDelegatingToGateway(
	ctx context.Context,
	gatewayAddress string,
) ([]string, error) {
	// DISCUSS: remove this call: pass to this function the list of Application structs, which can be obtained separately using the ApplicationClient.
	//	It can be composed using other basic components of the SDK, e.g. get all the applications, get the latest block height, etc.
	//	If this specific sequence of using basic components of the SDK occurs frequently enough that summarizing all the steps in
	//		a single function call is desirable, one possible option could be defining helper functions.
	allApplications, err := sdk.ApplicationLister.GetAllApplications(ctx)
	if err != nil {
		return nil, err
	}

	currentHeight, err := sdk.blockClient.GetLatestBlockHeight(ctx)
	if err != nil {
		return nil, err
	}

	params, err := sdk.paramsClient.GetParams(ctx)
	if err != nil {
		return nil, err
	}

	gatewayDelegatingApplications := make([]string, 0)
	for _, application := range allApplications {
		// Get the gateways that are currently delegated to the application
		// at the current height and check if the given gateway address is in the list.
		gatewaysDelegatedTo := rings.GetRingAddressesAtBlock(params, &application, currentHeight)
		if slices.Contains(gatewaysDelegatedTo, gatewayAddress) {
			// The application is delegating to the given gateway address, add it to the list.
			gatewayDelegatingApplications = append(gatewayDelegatingApplications, application.Address)
		}
	}

	return gatewayDelegatingApplications, nil
}

// SendRelay signs and sends a relay request to the given supplier endpoint
// with the given request body, method, and headers. It returns the relay
// response after verifying the supplier's signature.
func (sdk *ShannonSDK) SendRelay(
	ctx context.Context,
	sessionSupplierEndpoint *types.SingleSupplierEndpoint,
	requestBz []byte,
) (relayResponse *servicetypes.RelayResponse, err error) {
	if err := sessionSupplierEndpoint.SessionHeader.ValidateBasic(); err != nil {
		return nil, err
	}

	relayRequest := &servicetypes.RelayRequest{
		Meta: servicetypes.RelayRequestMetadata{
			SessionHeader: sessionSupplierEndpoint.SessionHeader,
			Signature:     nil,
		},
		Payload: requestBz,
	}

	relayRequestSig, err := sdk.signRelayRequest(ctx, relayRequest)
	if err != nil {
		return nil, err
	}

	relayRequest.Meta.Signature = relayRequestSig

	relayRequestBz, err := relayRequest.Marshal()
	if err != nil {
		return nil, err
	}

	relayResponseBz, err := sdk.relayClient.SendRequest(
		ctx,
		sessionSupplierEndpoint.Url,
		relayRequestBz,
	)
	if err != nil {
		return nil, err
	}

	relayResponse = &servicetypes.RelayResponse{}
	if err := relayResponse.Unmarshal(relayResponseBz); err != nil {
		return nil, err
	}

	if err := relayResponse.ValidateBasic(); err != nil {
		return nil, err
	}

	supplierPubKey, err := sdk.accountClient.GetPubKeyFromAddress(
		ctx,
		sessionSupplierEndpoint.SupplierAddress,
	)
	if err != nil {
		return nil, err
	}

	if err := relayResponse.VerifySupplierSignature(supplierPubKey); err != nil {
		return nil, err
	}

	return relayResponse, nil
}

// signRelayRequest signs the given relay request using the signer's private key
// and the application's ring signature.
func (sdk *ShannonSDK) signRelayRequest(
	ctx context.Context,
	relayRequest *servicetypes.RelayRequest,
) (signature []byte, err error) {
	appAddress := relayRequest.GetMeta().SessionHeader.GetApplicationAddress()

	appRing, err := sdk.getRingForApplicationAddress(ctx, appAddress)
	if err != nil {
		return nil, err
	}

	signableBz, err := relayRequest.GetSignableBytesHash()
	if err != nil {
		return nil, err
	}

	signerPrivKeyBz, err := hex.DecodeString(sdk.signer.GetPrivateKeyHex())
	if err != nil {
		return nil, err
	}

	signerPrivKey, err := ring.Secp256k1().DecodeToScalar(signerPrivKeyBz)
	if err != nil {
		return nil, err
	}

	ringSig, err := appRing.Sign(signableBz, signerPrivKey)
	if err != nil {
		return nil, err
	}

	return ringSig.Serialize()
}

// getRingForApplicationAddress returns the ring for the given application address.
// The ring is created using the application's public key and the public keys of
// the gateways that are currently delegated from the application.
func (sdk *ShannonSDK) getRingForApplicationAddress(
	ctx context.Context,
	appAddress string,
) (addressRing *ring.Ring, err error) {
	// DISCUSS: It may be a good idea to remove this call, and pass the application struct to this function, instead of an address.
	application, err := sdk.ApplicationLister.GetApplication(ctx, appAddress)
	if err != nil {
		return nil, err
	}

	latestHeight, err := sdk.blockClient.GetLatestBlockHeight(ctx)
	if err != nil {
		return nil, err
	}

	params, err := sdk.paramsClient.GetParams(ctx)
	if err != nil {
		return nil, err
	}

	// Get the current gateway addresses that are delegated from the application
	// at the latest height.
	currentGatewayAddresses := rings.GetRingAddressesAtBlock(params, &application, latestHeight)

	ringAddresses := make([]string, 0)
	ringAddresses = append(ringAddresses, application.Address)

	// If there are no current gateway addresses, use the application address as the ring address.
	if len(currentGatewayAddresses) == 0 {
		ringAddresses = append(ringAddresses, application.Address)
	} else {
		ringAddresses = append(ringAddresses, currentGatewayAddresses...)
	}

	curve := ring_secp256k1.NewCurve()
	ringPoints := make([]ringtypes.Point, 0, len(ringAddresses))

	// Create a ring point for each address.
	for _, address := range ringAddresses {
		pubKey, err := sdk.accountClient.GetPubKeyFromAddress(ctx, address)
		if err != nil {
			return nil, err
		}

		point, err := curve.DecodeToPoint(pubKey.Bytes())
		if err != nil {
			return nil, err
		}

		ringPoints = append(ringPoints, point)
	}

	return ring.NewFixedKeyRingFromPublicKeys(ring_secp256k1.NewCurve(), ringPoints)
}
