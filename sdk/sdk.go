package sdk

import (
	"context"
	"encoding/hex"
	"slices"

	ring_secp256k1 "github.com/athanorlabs/go-dleq/secp256k1"
	ringtypes "github.com/athanorlabs/go-dleq/types"
	"github.com/noot/ring-go"
	"github.com/pokt-network/poktroll/pkg/crypto/rings"
	"github.com/pokt-network/poktroll/x/service/types"
)

// ShannonSDK is the main struct for the SDK that will be used by the service
// to interact with the Shannon network
type ShannonSDK struct {
	applicationClient ApplicationClient
	sessionClient     SessionClient
	accountClient     AccountClient
	blockClient       BlockClient
	relayClient       RelayClient
	signer            Signer
}

// NewShannonSDK creates a new ShannonSDK instance with the given clients and signer.
// The clients are used to interact with the Shannon network.
// The signer is used to sign the relay requests.
func NewShannonSDK(
	applicationClient ApplicationClient,
	sessionClient SessionClient,
	accountClient AccountClient,
	blockClient BlockClient,
	relayClient RelayClient,
	signer Signer,
) (*ShannonSDK, error) {
	return &ShannonSDK{
		applicationClient: applicationClient,
		sessionClient:     sessionClient,
		accountClient:     accountClient,
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
) (sessionSuppliers *SessionSuppliers, err error) {
	latestHeight, err := sdk.blockClient.GetLatestBlockHeight(ctx)
	if err != nil {
		return nil, err
	}

	currentSession, err := sdk.sessionClient.GetSession(ctx, appAddress, serviceId, latestHeight)
	if err != nil {
		return nil, err
	}

	sessionSuppliers = &SessionSuppliers{
		Session:            currentSession,
		SuppliersEndpoints: make([]*SingleSupplierEndpoint, 0),
	}

	for _, supplier := range currentSession.Suppliers {
		for _, service := range supplier.Services {
			if service.Service.Id != serviceId {
				continue
			}

			for _, endpoint := range service.Endpoints {
				sessionSuppliers.SuppliersEndpoints = append(
					sessionSuppliers.SuppliersEndpoints,
					&SingleSupplierEndpoint{
						RpcType:         endpoint.RpcType,
						Url:             endpoint.Url,
						SupplierAddress: supplier.Address,
						Header:          currentSession.Header,
					},
				)
			}
		}
	}

	return sessionSuppliers, nil
}

// GetGatewayDelegatingApplications returns the applications that are delegating
// to the given gateway address.
func (sdk *ShannonSDK) GetGatewayDelegatingApplications(
	ctx context.Context,
	gatewayAddress string,
) ([]string, error) {
	allApplications, err := sdk.applicationClient.GetAllApplications(ctx)
	if err != nil {
		return nil, err
	}

	currentHeight, err := sdk.blockClient.GetLatestBlockHeight(ctx)
	if err != nil {
		return nil, err
	}

	gatewayDelegatingApplications := make([]string, 0)
	for _, application := range allApplications {
		// Get the gateways that are currently delegated to the application
		// at the current height and check if the given gateway address is in the list.
		gatewaysDelegatedTo := rings.GetRingAddressesAtBlock(&application, currentHeight)
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
	sessionSupplierEndpoint *SingleSupplierEndpoint,
	requestBody []byte,
	method string,
	requestHeaders map[string][]string,
) (relayResponse *types.RelayResponse, err error) {
	if err := sessionSupplierEndpoint.Header.ValidateBasic(); err != nil {
		return nil, err
	}

	relayRequest := &types.RelayRequest{
		Meta: types.RelayRequestMetadata{
			SessionHeader: sessionSupplierEndpoint.Header,
			Signature:     nil,
		},
		Payload: requestBody,
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
		method, requestHeaders,
	)
	if err != nil {
		return nil, err
	}

	relayResponse = &types.RelayResponse{}
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
	relayRequest *types.RelayRequest,
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
	application, err := sdk.applicationClient.GetApplication(ctx, appAddress)
	if err != nil {
		return nil, err
	}

	latestHeight, err := sdk.blockClient.GetLatestBlockHeight(ctx)
	if err != nil {
		return nil, err
	}

	// Get the current gateway addresses that are delegated from the application
	// at the latest height.
	currentGatewayAddresses := rings.GetRingAddressesAtBlock(&application, latestHeight)

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
