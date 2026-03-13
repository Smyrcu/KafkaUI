package kafka

import (
	"context"
	"fmt"

	"github.com/twmb/franz-go/pkg/kmsg"
)

// ACLEntry represents a single Kafka ACL binding.
type ACLEntry struct {
	ResourceType string `json:"resourceType"` // TOPIC, GROUP, CLUSTER, TRANSACTIONAL_ID
	ResourceName string `json:"resourceName"`
	PatternType  string `json:"patternType"` // LITERAL, PREFIXED
	Principal    string `json:"principal"`
	Host         string `json:"host"`
	Operation    string `json:"operation"`  // READ, WRITE, CREATE, DELETE, ALTER, DESCRIBE, ALL, etc.
	Permission   string `json:"permission"` // ALLOW, DENY
}

// ListACLs retrieves all ACL bindings from the Kafka cluster.
func (c *Client) ListACLs(ctx context.Context) ([]ACLEntry, error) {
	req := kmsg.NewDescribeACLsRequest()
	req.ResourceType = kmsg.ACLResourceTypeAny
	req.ResourcePatternType = kmsg.ACLResourcePatternTypeAny
	req.Operation = kmsg.ACLOperationAny
	req.PermissionType = kmsg.ACLPermissionTypeAny
	req.ResourceName = nil
	req.Principal = nil
	req.Host = nil

	resp, err := c.raw.Request(ctx, &req)
	if err != nil {
		return nil, fmt.Errorf("describing ACLs: %w", err)
	}

	aclResp, ok := resp.(*kmsg.DescribeACLsResponse)
	if !ok {
		return nil, fmt.Errorf("unexpected response type for DescribeACLs")
	}

	if aclResp.ErrorCode != 0 {
		return nil, fmt.Errorf("DescribeACLs failed with %s", kafkaErrMsg(aclResp.ErrorCode, aclResp.ErrorMessage))
	}

	var entries []ACLEntry
	for _, resource := range aclResp.Resources {
		for _, acl := range resource.ACLs {
			entries = append(entries, ACLEntry{
				ResourceType: resourceTypeToString(resource.ResourceType),
				ResourceName: resource.ResourceName,
				PatternType:  patternTypeToString(resource.ResourcePatternType),
				Principal:    acl.Principal,
				Host:         acl.Host,
				Operation:    operationToString(acl.Operation),
				Permission:   permissionToString(acl.PermissionType),
			})
		}
	}

	if entries == nil {
		entries = []ACLEntry{}
	}

	return entries, nil
}

// CreateACL creates a new ACL binding in the Kafka cluster.
func (c *Client) CreateACL(ctx context.Context, entry ACLEntry) error {
	req := kmsg.NewCreateACLsRequest()
	creation := kmsg.NewCreateACLsRequestCreation()
	creation.ResourceType = stringToResourceType(entry.ResourceType)
	creation.ResourceName = entry.ResourceName
	creation.ResourcePatternType = stringToPatternType(entry.PatternType)
	creation.Principal = entry.Principal
	creation.Host = entry.Host
	creation.Operation = stringToOperation(entry.Operation)
	creation.PermissionType = stringToPermission(entry.Permission)
	req.Creations = append(req.Creations, creation)

	resp, err := c.raw.Request(ctx, &req)
	if err != nil {
		return fmt.Errorf("creating ACL: %w", err)
	}

	createResp, ok := resp.(*kmsg.CreateACLsResponse)
	if !ok {
		return fmt.Errorf("unexpected response type for CreateACLs")
	}

	for _, result := range createResp.Results {
		if result.ErrorCode != 0 {
			return fmt.Errorf("CreateACL failed with %s", kafkaErrMsg(result.ErrorCode, result.ErrorMessage))
		}
	}

	return nil
}

// DeleteACL deletes ACL bindings matching the given entry from the Kafka cluster.
func (c *Client) DeleteACL(ctx context.Context, entry ACLEntry) error {
	req := kmsg.NewDeleteACLsRequest()
	filter := kmsg.NewDeleteACLsRequestFilter()
	filter.ResourceType = stringToResourceType(entry.ResourceType)
	filter.ResourceName = &entry.ResourceName
	filter.ResourcePatternType = stringToPatternType(entry.PatternType)
	filter.Principal = &entry.Principal
	filter.Host = &entry.Host
	filter.Operation = stringToOperation(entry.Operation)
	filter.PermissionType = stringToPermission(entry.Permission)
	req.Filters = append(req.Filters, filter)

	resp, err := c.raw.Request(ctx, &req)
	if err != nil {
		return fmt.Errorf("deleting ACL: %w", err)
	}

	deleteResp, ok := resp.(*kmsg.DeleteACLsResponse)
	if !ok {
		return fmt.Errorf("unexpected response type for DeleteACLs")
	}

	for _, result := range deleteResp.Results {
		if result.ErrorCode != 0 {
			return fmt.Errorf("DeleteACL failed with %s", kafkaErrMsg(result.ErrorCode, result.ErrorMessage))
		}
	}

	return nil
}

// resourceTypeToString converts a kmsg ACL resource type constant to its string representation.
func resourceTypeToString(rt kmsg.ACLResourceType) string {
	switch rt {
	case kmsg.ACLResourceTypeTopic:
		return "TOPIC"
	case kmsg.ACLResourceTypeGroup:
		return "GROUP"
	case kmsg.ACLResourceTypeCluster:
		return "CLUSTER"
	case kmsg.ACLResourceTypeTransactionalId:
		return "TRANSACTIONAL_ID"
	default:
		return "UNKNOWN"
	}
}

// stringToResourceType converts a string to a kmsg ACL resource type constant.
func stringToResourceType(s string) kmsg.ACLResourceType {
	switch s {
	case "TOPIC":
		return kmsg.ACLResourceTypeTopic
	case "GROUP":
		return kmsg.ACLResourceTypeGroup
	case "CLUSTER":
		return kmsg.ACLResourceTypeCluster
	case "TRANSACTIONAL_ID":
		return kmsg.ACLResourceTypeTransactionalId
	default:
		return kmsg.ACLResourceTypeAny
	}
}

// patternTypeToString converts a kmsg ACL resource pattern type constant to its string representation.
func patternTypeToString(pt kmsg.ACLResourcePatternType) string {
	switch pt {
	case kmsg.ACLResourcePatternTypeLiteral:
		return "LITERAL"
	case kmsg.ACLResourcePatternTypePrefixed:
		return "PREFIXED"
	default:
		return "UNKNOWN"
	}
}

// stringToPatternType converts a string to a kmsg ACL resource pattern type constant.
func stringToPatternType(s string) kmsg.ACLResourcePatternType {
	switch s {
	case "LITERAL":
		return kmsg.ACLResourcePatternTypeLiteral
	case "PREFIXED":
		return kmsg.ACLResourcePatternTypePrefixed
	default:
		return kmsg.ACLResourcePatternTypeAny
	}
}

// operationToString converts a kmsg ACL operation constant to its string representation.
func operationToString(op kmsg.ACLOperation) string {
	switch op {
	case kmsg.ACLOperationRead:
		return "READ"
	case kmsg.ACLOperationWrite:
		return "WRITE"
	case kmsg.ACLOperationCreate:
		return "CREATE"
	case kmsg.ACLOperationDelete:
		return "DELETE"
	case kmsg.ACLOperationAlter:
		return "ALTER"
	case kmsg.ACLOperationDescribe:
		return "DESCRIBE"
	case kmsg.ACLOperationClusterAction:
		return "CLUSTER_ACTION"
	case kmsg.ACLOperationDescribeConfigs:
		return "DESCRIBE_CONFIGS"
	case kmsg.ACLOperationAlterConfigs:
		return "ALTER_CONFIGS"
	case kmsg.ACLOperationIdempotentWrite:
		return "IDEMPOTENT_WRITE"
	case kmsg.ACLOperationAll:
		return "ALL"
	default:
		return "UNKNOWN"
	}
}

// stringToOperation converts a string to a kmsg ACL operation constant.
func stringToOperation(s string) kmsg.ACLOperation {
	switch s {
	case "READ":
		return kmsg.ACLOperationRead
	case "WRITE":
		return kmsg.ACLOperationWrite
	case "CREATE":
		return kmsg.ACLOperationCreate
	case "DELETE":
		return kmsg.ACLOperationDelete
	case "ALTER":
		return kmsg.ACLOperationAlter
	case "DESCRIBE":
		return kmsg.ACLOperationDescribe
	case "CLUSTER_ACTION":
		return kmsg.ACLOperationClusterAction
	case "DESCRIBE_CONFIGS":
		return kmsg.ACLOperationDescribeConfigs
	case "ALTER_CONFIGS":
		return kmsg.ACLOperationAlterConfigs
	case "IDEMPOTENT_WRITE":
		return kmsg.ACLOperationIdempotentWrite
	case "ALL":
		return kmsg.ACLOperationAll
	default:
		return kmsg.ACLOperationAny
	}
}

// permissionToString converts a kmsg ACL permission type constant to its string representation.
func permissionToString(pt kmsg.ACLPermissionType) string {
	switch pt {
	case kmsg.ACLPermissionTypeAllow:
		return "ALLOW"
	case kmsg.ACLPermissionTypeDeny:
		return "DENY"
	default:
		return "UNKNOWN"
	}
}

// stringToPermission converts a string to a kmsg ACL permission type constant.
func stringToPermission(s string) kmsg.ACLPermissionType {
	switch s {
	case "ALLOW":
		return kmsg.ACLPermissionTypeAllow
	case "DENY":
		return kmsg.ACLPermissionTypeDeny
	default:
		return kmsg.ACLPermissionTypeAny
	}
}
