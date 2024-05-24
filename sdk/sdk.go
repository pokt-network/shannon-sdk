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

type ShannonSDK struct {
	applicationClient ApplicationClient
	sessionClient     SessionClient
	accountClient     AccountClient
	blockClient       BlockClient
	relayClient       RelayClient
	signer            Signer
}

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

func (sdk *ShannonSDK) GetSessionSupplierEndpoints(
	ctx context.Context,
	appAddress string,
	serviceId string,
) (sessionSuppliers *SessionSuppliers, err error) {
	height, err := sdk.blockClient.GetLatestBlockHeight(ctx)
	if err != nil {
		return nil, err
	}

	currentSession, err := sdk.sessionClient.GetSession(ctx, appAddress, serviceId, height)
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
		gatewaysDelegatedTo := rings.GetRingAddressesAtBlock(&application, currentHeight)
		if slices.Contains(gatewaysDelegatedTo, gatewayAddress) {
			gatewayDelegatingApplications = append(gatewayDelegatingApplications, application.Address)
		}
	}

	return gatewayDelegatingApplications, nil
}

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

	relayResponseBz, err := sdk.relayClient.Do(
		ctx,
		sessionSupplierEndpoint.Url,
		relayRequestBz,
		method, requestHeaders,
	)
	if err != nil {
		return nil, err
	}

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

func (sdk *ShannonSDK) getRingForApplicationAddress(
	ctx context.Context,
	appAddress string,
) (addressRing *ring.Ring, err error) {
	application, err := sdk.applicationClient.GetApplication(ctx, appAddress)
	if err != nil {
		return nil, err
	}

	height, err := sdk.blockClient.GetLatestBlockHeight(ctx)
	if err != nil {
		return nil, err
	}

	currentGatewayAddresses := rings.GetRingAddressesAtBlock(&application, height)

	ringAddresses := make([]string, 0, 1+len(currentGatewayAddresses))
	ringAddresses = append(ringAddresses, application.Address)

	if len(currentGatewayAddresses) == 0 {
		ringAddresses = append(ringAddresses, application.Address)
	} else {
		ringAddresses = append(ringAddresses, currentGatewayAddresses...)
	}

	curve := ring_secp256k1.NewCurve()
	ringPoints := make([]ringtypes.Point, 0, len(ringAddresses))

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
