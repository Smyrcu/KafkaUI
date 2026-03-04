package kafka

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/sha512"
	"fmt"
	"hash"

	"github.com/twmb/franz-go/pkg/kmsg"
	"golang.org/x/crypto/pbkdf2"
)

// ScramUser represents a Kafka SCRAM user credential.
type ScramUser struct {
	Name       string `json:"name"`
	Mechanism  string `json:"mechanism"` // SCRAM-SHA-256 or SCRAM-SHA-512
	Iterations int32  `json:"iterations"`
}

// UpsertScramUserRequest contains the data needed to create or update a SCRAM user.
type UpsertScramUserRequest struct {
	Name       string `json:"name"`
	Password   string `json:"password"`
	Mechanism  string `json:"mechanism"`  // SCRAM-SHA-256 or SCRAM-SHA-512
	Iterations int32  `json:"iterations"` // default 4096 if 0
}

// ListScramUsers returns all SCRAM user credentials in the cluster.
func (c *Client) ListScramUsers(ctx context.Context) ([]ScramUser, error) {
	req := kmsg.NewDescribeUserSCRAMCredentialsRequest()
	// Empty Users slice = describe all users

	resp, err := c.raw.Request(ctx, &req)
	if err != nil {
		return nil, fmt.Errorf("describing SCRAM credentials: %w", err)
	}

	descResp, ok := resp.(*kmsg.DescribeUserSCRAMCredentialsResponse)
	if !ok {
		return nil, fmt.Errorf("unexpected response type for DescribeUserSCRAMCredentials")
	}

	if descResp.ErrorCode != 0 {
		msg := ""
		if descResp.ErrorMessage != nil {
			msg = *descResp.ErrorMessage
		}
		return nil, fmt.Errorf("DescribeUserSCRAMCredentials failed with error code %d: %s", descResp.ErrorCode, msg)
	}

	var users []ScramUser
	for _, result := range descResp.Results {
		if result.ErrorCode != 0 {
			continue // skip users with errors (e.g. RESOURCE_NOT_FOUND)
		}
		for _, cred := range result.CredentialInfos {
			users = append(users, ScramUser{
				Name:       result.User,
				Mechanism:  scramMechanismToString(cred.Mechanism),
				Iterations: cred.Iterations,
			})
		}
	}

	if users == nil {
		users = []ScramUser{}
	}

	return users, nil
}

// UpsertScramUser creates or updates a SCRAM credential for a user.
func (c *Client) UpsertScramUser(ctx context.Context, u UpsertScramUserRequest) error {
	if u.Iterations == 0 {
		u.Iterations = 4096
	}

	mechanism := stringToScramMechanism(u.Mechanism)
	if mechanism == 0 {
		return fmt.Errorf("unsupported SCRAM mechanism: %s", u.Mechanism)
	}

	salt := make([]byte, 32)
	if _, err := rand.Read(salt); err != nil {
		return fmt.Errorf("generating salt: %w", err)
	}

	var hashFunc func() hash.Hash
	var keyLen int
	switch mechanism {
	case 1: // SCRAM-SHA-256
		hashFunc = sha256.New
		keyLen = 32
	case 2: // SCRAM-SHA-512
		hashFunc = sha512.New
		keyLen = 64
	}

	saltedPassword := pbkdf2.Key([]byte(u.Password), salt, int(u.Iterations), keyLen, hashFunc)

	req := kmsg.NewAlterUserSCRAMCredentialsRequest()
	upsertion := kmsg.NewAlterUserSCRAMCredentialsRequestUpsertion()
	upsertion.Name = u.Name
	upsertion.Mechanism = mechanism
	upsertion.Iterations = u.Iterations
	upsertion.Salt = salt
	upsertion.SaltedPassword = saltedPassword
	req.Upsertions = append(req.Upsertions, upsertion)

	resp, err := c.raw.Request(ctx, &req)
	if err != nil {
		return fmt.Errorf("altering SCRAM credentials: %w", err)
	}

	alterResp, ok := resp.(*kmsg.AlterUserSCRAMCredentialsResponse)
	if !ok {
		return fmt.Errorf("unexpected response type for AlterUserSCRAMCredentials")
	}

	for _, result := range alterResp.Results {
		if result.ErrorCode != 0 {
			msg := ""
			if result.ErrorMessage != nil {
				msg = *result.ErrorMessage
			}
			return fmt.Errorf("AlterUserSCRAMCredentials failed for user %q: error code %d: %s", result.User, result.ErrorCode, msg)
		}
	}

	return nil
}

// DeleteScramUser removes a SCRAM credential for a user.
func (c *Client) DeleteScramUser(ctx context.Context, name string, mechanism string) error {
	mech := stringToScramMechanism(mechanism)
	if mech == 0 {
		return fmt.Errorf("unsupported SCRAM mechanism: %s", mechanism)
	}

	req := kmsg.NewAlterUserSCRAMCredentialsRequest()
	deletion := kmsg.NewAlterUserSCRAMCredentialsRequestDeletion()
	deletion.Name = name
	deletion.Mechanism = mech
	req.Deletions = append(req.Deletions, deletion)

	resp, err := c.raw.Request(ctx, &req)
	if err != nil {
		return fmt.Errorf("deleting SCRAM credentials: %w", err)
	}

	alterResp, ok := resp.(*kmsg.AlterUserSCRAMCredentialsResponse)
	if !ok {
		return fmt.Errorf("unexpected response type for AlterUserSCRAMCredentials")
	}

	for _, result := range alterResp.Results {
		if result.ErrorCode != 0 {
			msg := ""
			if result.ErrorMessage != nil {
				msg = *result.ErrorMessage
			}
			return fmt.Errorf("DeleteUserSCRAMCredentials failed for user %q: error code %d: %s", result.User, result.ErrorCode, msg)
		}
	}

	return nil
}

func scramMechanismToString(m int8) string {
	switch m {
	case 1:
		return "SCRAM-SHA-256"
	case 2:
		return "SCRAM-SHA-512"
	default:
		return "UNKNOWN"
	}
}

func stringToScramMechanism(s string) int8 {
	switch s {
	case "SCRAM-SHA-256":
		return 1
	case "SCRAM-SHA-512":
		return 2
	default:
		return 0
	}
}
