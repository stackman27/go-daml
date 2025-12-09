package topology_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/noders-team/go-daml/pkg/model"
	"github.com/noders-team/go-daml/pkg/service/topology"
	"github.com/noders-team/go-daml/pkg/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func TestOnboardExternalParty(t *testing.T) {
	ctx := context.Background()

	conn, err := grpc.Dial(
		testutil.GetAdminAddr(),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	defer conn.Close()

	writeClient := topology.NewTopologyManagerWriteClient(conn)

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)

	pubKey := model.PublicKey{
		Format: 4,
		Key:    []byte(pub),
	}

	namespace := computeFingerprint(pubKey)
	partyID := "TestParty::" + namespace

	prepareReq := &topology.PrepareOnboardingTransactionsRequest{
		PartyID:        partyID,
		Namespace:      namespace,
		PublicKeys:     []model.PublicKey{pubKey},
		ParticipantUID: "PAR::participant1::1220",
	}

	prepareResp, err := writeClient.PrepareOnboardingTransactions(ctx, prepareReq)
	require.NoError(t, err)
	assert.NotNil(t, prepareResp)
	assert.NotEmpty(t, prepareResp.NamespaceDelegationTxBytes)
	assert.NotEmpty(t, prepareResp.PartyToKeyMappingTxBytes)
	assert.NotEmpty(t, prepareResp.PartyToParticipantTxBytes)

	nsSig := signTransaction(prepareResp.NamespaceDelegationTxBytes, priv)
	partyKeyMappingSig := signTransaction(prepareResp.PartyToKeyMappingTxBytes, priv)
	partyParticipantSig := signTransaction(prepareResp.PartyToParticipantTxBytes, priv)

	nsFingerprint := computeFingerprint(pubKey)

	onboardReq := &topology.OnboardExternalPartyRequest{
		NamespaceDelegationTxBytes: prepareResp.NamespaceDelegationTxBytes,
		NamespaceDelegationSignatures: []model.TopologyTransactionSignature{
			{
				SignedBy:        nsFingerprint,
				Signature:       nsSig,
				SignatureFormat: 1,
			},
		},
		PartyToKeyMappingTxBytes: prepareResp.PartyToKeyMappingTxBytes,
		PartyToKeyMappingSignatures: []model.TopologyTransactionSignature{
			{
				SignedBy:        nsFingerprint,
				Signature:       partyKeyMappingSig,
				SignatureFormat: 1,
			},
		},
		PartyToParticipantTxBytes: prepareResp.PartyToParticipantTxBytes,
		PartyToParticipantSignatures: []model.TopologyTransactionSignature{
			{
				SignedBy:        nsFingerprint,
				Signature:       partyParticipantSig,
				SignatureFormat: 1,
			},
		},
		Store: nil,
	}

	onboardResp, err := writeClient.OnboardExternalParty(ctx, onboardReq)
	require.NoError(t, err)
	require.NotNil(t, onboardResp)
	assert.True(t, onboardResp.Success)
}

func TestPrepareOnboardingTransactions(t *testing.T) {
	ctx := context.Background()

	conn, err := grpc.Dial(testutil.GetAdminAddr(),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	defer conn.Close()

	writeClient := topology.NewTopologyManagerWriteClient(conn)

	tests := []struct {
		name          string
		partyID       string
		participantID string
		keyCount      int
	}{
		{
			name:          "single key transaction preparation",
			partyID:       "test1",
			participantID: "PAR::participant1::1220",
			keyCount:      1,
		},
		{
			name:          "multiple keys transaction preparation",
			partyID:       "test2",
			participantID: "PAR::participant1::1220",
			keyCount:      2,
		},
		{
			name:          "long party name",
			partyID:       "test3",
			participantID: "PAR::participant1::1220",
			keyCount:      1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pubKeys := make([]model.PublicKey, tt.keyCount)
			privKeys := make([]ed25519.PrivateKey, tt.keyCount)

			for i := 0; i < tt.keyCount; i++ {
				pub, priv, err := ed25519.GenerateKey(rand.Reader)
				require.NoError(t, err)

				pubKeys[i] = model.PublicKey{
					Format: 4,
					Key:    []byte(pub),
				}
				privKeys[i] = priv
			}

			namespace := computeFingerprint(pubKeys[0])

			prepareReq := &topology.PrepareOnboardingTransactionsRequest{
				PartyID:        tt.partyID + "::" + namespace,
				Namespace:      namespace,
				PublicKeys:     pubKeys,
				ParticipantUID: tt.participantID,
			}

			prepareResp, err := writeClient.PrepareOnboardingTransactions(ctx, prepareReq)
			require.NoError(t, err)
			assert.NotNil(t, prepareResp)
			assert.NotEmpty(t, prepareResp.NamespaceDelegationTxBytes)
			assert.NotEmpty(t, prepareResp.PartyToKeyMappingTxBytes)
			assert.NotEmpty(t, prepareResp.PartyToParticipantTxBytes)

			nsSig := signTransaction(prepareResp.NamespaceDelegationTxBytes, privKeys[0])
			assert.NotEmpty(t, nsSig)
			assert.Equal(t, 64, len(nsSig))

			partyKeyMappingSig := signTransaction(prepareResp.PartyToKeyMappingTxBytes, privKeys[0])
			assert.NotEmpty(t, partyKeyMappingSig)
			assert.Equal(t, 64, len(partyKeyMappingSig))

			partyParticipantSig := signTransaction(prepareResp.PartyToParticipantTxBytes, privKeys[0])
			assert.NotEmpty(t, partyParticipantSig)
			assert.Equal(t, 64, len(partyParticipantSig))

			nsFingerprint := computeFingerprint(pubKeys[0])
			assert.NotEmpty(t, nsFingerprint)
			assert.Equal(t, 64, len(nsFingerprint))
		})
	}
}

func signTransaction(txBytes []byte, privKey ed25519.PrivateKey) []byte {
	hash := sha256.Sum256(txBytes)
	signature := ed25519.Sign(privKey, hash[:])
	return signature
}

func computeFingerprint(pubKey model.PublicKey) string {
	hash := sha256.Sum256(pubKey.Key)
	return hex.EncodeToString(hash[:])
}
