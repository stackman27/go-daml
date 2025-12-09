package topology

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	cryptov30 "github.com/digital-asset/dazl-client/v8/go/api/com/digitalasset/canton/crypto/v30"
	protov30 "github.com/digital-asset/dazl-client/v8/go/api/com/digitalasset/canton/protocol/v30"
	"github.com/noders-team/go-daml/pkg/model"
	"google.golang.org/protobuf/proto"
)

type PrepareOnboardingTransactionsRequest struct {
	PartyID        string
	Namespace      string
	PublicKeys     []model.PublicKey
	ParticipantUID string
}

type PrepareOnboardingTransactionsResponse struct {
	NamespaceDelegationTxBytes []byte
	PartyToKeyMappingTxBytes   []byte
	PartyToParticipantTxBytes  []byte
}

type OnboardExternalPartyRequest struct {
	NamespaceDelegationTxBytes    []byte
	NamespaceDelegationSignatures []model.TopologyTransactionSignature
	PartyToKeyMappingTxBytes      []byte
	PartyToKeyMappingSignatures   []model.TopologyTransactionSignature
	PartyToParticipantTxBytes     []byte
	PartyToParticipantSignatures  []model.TopologyTransactionSignature
	Store                         *model.StoreID
}

type OnboardExternalPartyResponse struct {
	Success bool
}

func (c *topologyManagerWrite) PrepareOnboardingTransactions(ctx context.Context, req *PrepareOnboardingTransactionsRequest) (*PrepareOnboardingTransactionsResponse, error) {
	if req.PartyID == "" {
		return nil, fmt.Errorf("partyID is required")
	}
	if len(req.PublicKeys) == 0 {
		return nil, fmt.Errorf("at least one public key is required")
	}
	if req.ParticipantUID == "" {
		return nil, fmt.Errorf("participantUID is required")
	}

	namespace := req.Namespace
	if namespace == "" {
		namespace = buildPartyKey(req.PublicKeys[0])
	}

	namespaceDelegationTx, err := buildNamespaceDelegationTransaction(namespace, req.PublicKeys[0])
	if err != nil {
		return nil, fmt.Errorf("failed to build namespace delegation transaction: %w", err)
	}

	partyToKeyTx, err := buildPartyToKeyMappingTransaction(req.PartyID, req.PublicKeys)
	if err != nil {
		return nil, fmt.Errorf("failed to build party to key mapping transaction: %w", err)
	}

	partyToParticipantTx, err := buildPartyToParticipantTransaction(req.PartyID, req.ParticipantUID)
	if err != nil {
		return nil, fmt.Errorf("failed to build party to participant transaction: %w", err)
	}

	return &PrepareOnboardingTransactionsResponse{
		NamespaceDelegationTxBytes: namespaceDelegationTx,
		PartyToKeyMappingTxBytes:   partyToKeyTx,
		PartyToParticipantTxBytes:  partyToParticipantTx,
	}, nil
}

func (c *topologyManagerWrite) OnboardExternalParty(ctx context.Context, req *OnboardExternalPartyRequest) (*OnboardExternalPartyResponse, error) {
	if req.NamespaceDelegationTxBytes == nil || req.PartyToKeyMappingTxBytes == nil || req.PartyToParticipantTxBytes == nil {
		return nil, fmt.Errorf("all three transaction bytes are required")
	}

	transactions := []*model.SignedTopologyTransaction{
		{
			Transaction: req.NamespaceDelegationTxBytes,
			Signatures:  req.NamespaceDelegationSignatures,
			Proposal:    false,
		},
		{
			Transaction: req.PartyToKeyMappingTxBytes,
			Signatures:  req.PartyToKeyMappingSignatures,
			Proposal:    false,
		},
		{
			Transaction: req.PartyToParticipantTxBytes,
			Signatures:  req.PartyToParticipantSignatures,
			Proposal:    false,
		},
	}

	_, err := c.AddTransactions(ctx, &model.AddTransactionsRequest{
		Transactions: transactions,
		Store:        req.Store,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to add topology transactions: %w", err)
	}

	return &OnboardExternalPartyResponse{
		Success: true,
	}, nil
}

func buildNamespaceDelegationTransaction(namespace string, publicKey model.PublicKey) ([]byte, error) {
	mapping := &protov30.TopologyMapping{
		Mapping: &protov30.TopologyMapping_NamespaceDelegation{
			NamespaceDelegation: &protov30.NamespaceDelegation{
				Namespace: namespace,
				TargetKey: &cryptov30.SigningPublicKey{
					Format:    cryptov30.CryptoKeyFormat(publicKey.Format),
					PublicKey: publicKey.Key,
				},
				IsRootDelegation: true,
			},
		},
	}
	return serializeTopologyMapping(mapping)
}

func buildPartyToKeyMappingTransaction(partyID string, publicKeys []model.PublicKey) ([]byte, error) {
	keys := make([]*cryptov30.SigningPublicKey, len(publicKeys))
	for i, k := range publicKeys {
		keys[i] = &cryptov30.SigningPublicKey{
			Format:    cryptov30.CryptoKeyFormat(k.Format),
			PublicKey: k.Key,
		}
	}

	mapping := &protov30.TopologyMapping{
		Mapping: &protov30.TopologyMapping_PartyToKeyMapping{
			PartyToKeyMapping: &protov30.PartyToKeyMapping{
				Party:       partyID,
				Threshold:   1,
				SigningKeys: keys,
			},
		},
	}
	return serializeTopologyMapping(mapping)
}

func buildPartyToParticipantTransaction(partyID string, participantUID string) ([]byte, error) {
	mapping := &protov30.TopologyMapping{
		Mapping: &protov30.TopologyMapping_PartyToParticipant{
			PartyToParticipant: &protov30.PartyToParticipant{
				Party:     partyID,
				Threshold: 1,
				Participants: []*protov30.PartyToParticipant_HostingParticipant{
					{
						ParticipantUid: participantUID,
						Permission:     protov30.Enums_PARTICIPANT_PERMISSION_CONFIRMATION,
					},
				},
			},
		},
	}
	return serializeTopologyMapping(mapping)
}

func serializeTopologyMapping(mapping *protov30.TopologyMapping) ([]byte, error) {
	return proto.Marshal(mapping)
}

func buildPartyKey(publicKey model.PublicKey) string {
	hash := sha256.Sum256(publicKey.Key)
	return hex.EncodeToString(hash[:])
}
