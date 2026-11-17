package middleware

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	ddbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	snstypes "github.com/aws/aws-sdk-go-v2/service/sns/types"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/attribute"
)

// These tests use the real SDK input types so a renamed SDK field fails here rather than silently
// dropping an attribute.

func asMap(kvs []attribute.KeyValue) map[string]any {
	m := map[string]any{}
	for _, kv := range kvs {
		m[string(kv.Key)] = kv.Value.AsInterface()
	}
	return m
}

func TestServiceAttributes_DynamoDB(t *testing.T) {
	tests := []struct {
		name string
		in   any
		want map[string]any
	}{
		{"GetItem", &dynamodb.GetItemInput{TableName: aws.String("t"), ConsistentRead: aws.Bool(true), ProjectionExpression: aws.String("a,b")},
			map[string]any{"db.system.name": "aws.dynamodb", "aws.dynamodb.table_names": []string{"t"}, "aws.dynamodb.consistent_read": true, "aws.dynamodb.projection": "a,b"}},
		{"Query", &dynamodb.QueryInput{TableName: aws.String("t"), IndexName: aws.String("i"), Limit: aws.Int32(5), ScanIndexForward: aws.Bool(false), Select: ddbtypes.SelectCount},
			map[string]any{"db.system.name": "aws.dynamodb", "aws.dynamodb.table_names": []string{"t"}, "aws.dynamodb.index_name": "i", "aws.dynamodb.limit": int64(5), "aws.dynamodb.scan_forward": false, "aws.dynamodb.select": "COUNT"}},
		{"Scan", &dynamodb.ScanInput{TableName: aws.String("t"), Segment: aws.Int32(1), TotalSegments: aws.Int32(4)},
			map[string]any{"db.system.name": "aws.dynamodb", "aws.dynamodb.table_names": []string{"t"}, "aws.dynamodb.segment": int64(1), "aws.dynamodb.total_segments": int64(4)}},
		{"BatchGetItem", &dynamodb.BatchGetItemInput{RequestItems: map[string]ddbtypes.KeysAndAttributes{"b": {}, "a": {}}},
			map[string]any{"db.system.name": "aws.dynamodb", "aws.dynamodb.table_names": []string{"a", "b"}}},
		{"ListTables", &dynamodb.ListTablesInput{ExclusiveStartTableName: aws.String("x"), Limit: aws.Int32(2)},
			map[string]any{"db.system.name": "aws.dynamodb", "aws.dynamodb.exclusive_start_table": "x", "aws.dynamodb.limit": int64(2)}},
		{"PutItem nil table", &dynamodb.PutItemInput{},
			map[string]any{"db.system.name": "aws.dynamodb"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, asMap(serviceAttributes("DynamoDB", tt.in)))
		})
	}
}

func TestServiceAttributes_S3(t *testing.T) {
	got := asMap(serviceAttributes("S3", &s3.UploadPartCopyInput{
		Bucket: aws.String("b"), Key: aws.String("k"), CopySource: aws.String("src"), UploadId: aws.String("u"), PartNumber: aws.Int32(3),
	}))
	assert.Equal(t, map[string]any{
		"aws.s3.bucket": "b", "aws.s3.key": "k", "aws.s3.copy_source": "src", "aws.s3.upload_id": "u", "aws.s3.part_number": int64(3),
	}, got)
	assert.Empty(t, serviceAttributes("S3", &s3.ListBucketsInput{}))
}

func TestServiceAttributes_SQS(t *testing.T) {
	assert.Equal(t,
		map[string]any{"messaging.system": "aws_sqs", "server.address": "https://sqs/q"},
		asMap(serviceAttributes("SQS", &sqs.SendMessageInput{QueueUrl: aws.String("https://sqs/q")})))
	// nil QueueUrl must not panic (upstream otelaws dereferences without a check).
	assert.Equal(t, map[string]any{"messaging.system": "aws_sqs"},
		asMap(serviceAttributes("SQS", &sqs.ListQueuesInput{})))
	assert.NotPanics(t, func() { serviceAttributes("SQS", &sqs.SendMessageInput{}) })
}

func TestServiceAttributes_SNS(t *testing.T) {
	pub := asMap(serviceAttributes("SNS", &sns.PublishInput{TopicArn: aws.String("arn:aws:sns:eu-west-2:123:my-topic")}))
	assert.Equal(t, map[string]any{
		"messaging.system": "aws_sns", "messaging.destination.name": "my-topic",
		"messaging.operation.type": "send", "messaging.operation.name": "publish_input",
	}, pub)

	batch := asMap(serviceAttributes("SNS", &sns.PublishBatchInput{
		TopicArn:                   aws.String("arn:aws:sns:eu-west-2:123:t2"),
		PublishBatchRequestEntries: make([]snstypes.PublishBatchRequestEntry, 3),
	}))
	assert.Equal(t, "t2", batch["messaging.destination.name"])
	assert.Equal(t, "publish_batch_input", batch["messaging.operation.name"])
	assert.Equal(t, int64(3), batch["messaging.batch.message_count"])

	target := asMap(serviceAttributes("SNS", &sns.PublishInput{TargetArn: aws.String("arn:aws:sns:eu-west-2:123:ep")}))
	assert.Equal(t, "ep", target["messaging.destination.name"])
}

func TestServiceAttributes_SafeOnOddInputs(t *testing.T) {
	assert.NotPanics(t, func() {
		serviceAttributes("S3", nil)
		serviceAttributes("S3", (*s3.GetObjectInput)(nil))
		serviceAttributes("S3", "not a struct")
		serviceAttributes("Unknown", &s3.GetObjectInput{})
	})
	assert.Nil(t, serviceAttributes("Unknown", &s3.GetObjectInput{Bucket: aws.String("b")}))
}
