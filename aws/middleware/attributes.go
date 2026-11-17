package middleware

import (
	"reflect"
	"sort"
	"strings"

	"go.opentelemetry.io/otel/attribute"
)

// Service-specific span attributes, ported in spirit from the per-service builders of the deprecated
// upstream otelaws module (v0.72.0, Apache-2.0) — see the package comment and TD-003.
//
// Unlike upstream, these read well-known input fields by name using reflection instead of type-switching
// on SDK input types, so this module does not need to import the dynamodb, s3, sns or sqs SDK modules.
// A renamed SDK field would silently stop producing its attribute; attributes_test.go uses the real SDK
// input types to catch that. Not ported: DynamoDB index/attribute definitions and provisioned throughput,
// and the S3 DeleteObjects body.

// ServiceIDs as reported by the SDK (awsmiddleware.GetServiceID).
const (
	serviceDynamoDB = "DynamoDB"
	serviceS3       = "S3"
	serviceSQS      = "SQS"
	serviceSNS      = "SNS"
)

// serviceAttributes returns service-specific attributes for the given SDK service ID and operation input.
func serviceAttributes(serviceID string, params any) []attribute.KeyValue {
	if !isNonNilStructPtr(params) {
		return nil
	}
	switch serviceID {
	case serviceDynamoDB:
		return dynamoDBAttributes(params)
	case serviceS3:
		return s3Attributes(params)
	case serviceSQS:
		return sqsAttributes(params)
	case serviceSNS:
		return snsAttributes(params)
	}
	return nil
}

func dynamoDBAttributes(in any) []attribute.KeyValue {
	attrs := []attribute.KeyValue{attribute.String("db.system.name", "aws.dynamodb")}

	if t, ok := stringField(in, "TableName"); ok {
		attrs = append(attrs, attribute.StringSlice("aws.dynamodb.table_names", []string{t}))
	} else if names := mapKeysField(in, "RequestItems"); len(names) > 0 {
		attrs = append(attrs, attribute.StringSlice("aws.dynamodb.table_names", names))
	}
	if v, ok := boolField(in, "ConsistentRead"); ok {
		attrs = append(attrs, attribute.Bool("aws.dynamodb.consistent_read", v))
	}
	if v, ok := boolField(in, "ScanIndexForward"); ok {
		attrs = append(attrs, attribute.Bool("aws.dynamodb.scan_forward", v))
	}
	for field, key := range map[string]string{
		"ProjectionExpression":    "aws.dynamodb.projection",
		"IndexName":               "aws.dynamodb.index_name",
		"ExclusiveStartTableName": "aws.dynamodb.exclusive_start_table",
		"Select":                  "aws.dynamodb.select",
	} {
		if v, ok := stringField(in, field); ok {
			attrs = append(attrs, attribute.String(key, v))
		}
	}
	for field, key := range map[string]string{
		"Limit":         "aws.dynamodb.limit",
		"Segment":       "aws.dynamodb.segment",
		"TotalSegments": "aws.dynamodb.total_segments",
	} {
		if v, ok := intField(in, field); ok {
			attrs = append(attrs, attribute.Int64(key, v))
		}
	}
	return sortAttrs(attrs)
}

func s3Attributes(in any) []attribute.KeyValue {
	var attrs []attribute.KeyValue
	for field, key := range map[string]string{
		"Bucket":     "aws.s3.bucket",
		"Key":        "aws.s3.key",
		"CopySource": "aws.s3.copy_source",
		"UploadId":   "aws.s3.upload_id",
	} {
		if v, ok := stringField(in, field); ok {
			attrs = append(attrs, attribute.String(key, v))
		}
	}
	if v, ok := intField(in, "PartNumber"); ok {
		attrs = append(attrs, attribute.Int64("aws.s3.part_number", v))
	}
	return sortAttrs(attrs)
}

func sqsAttributes(in any) []attribute.KeyValue {
	attrs := []attribute.KeyValue{attribute.String("messaging.system", "aws_sqs")}
	// Matches upstream otelaws, which maps the queue URL to server.address.
	if v, ok := stringField(in, "QueueUrl"); ok {
		attrs = append(attrs, attribute.String("server.address", v))
	}
	return attrs
}

func snsAttributes(in any) []attribute.KeyValue {
	attrs := []attribute.KeyValue{attribute.String("messaging.system", "aws_sns")}

	var opName string
	switch reflect.TypeOf(in).Elem().Name() {
	case "PublishInput":
		opName = "publish_input"
	case "PublishBatchInput":
		opName = "publish_batch_input"
		if n, ok := lenField(in, "PublishBatchRequestEntries"); ok {
			attrs = append(attrs, attribute.Int("messaging.batch.message_count", n))
		}
	default:
		return attrs
	}

	dest := ""
	if arn, ok := stringField(in, "TopicArn"); ok {
		dest = lastARNSegment(arn)
	} else if arn, ok := stringField(in, "TargetArn"); ok {
		dest = lastARNSegment(arn)
	}
	return append(attrs,
		attribute.String("messaging.destination.name", dest),
		attribute.String("messaging.operation.type", "send"),
		attribute.String("messaging.operation.name", opName),
	)
}

func lastARNSegment(arn string) string { return arn[strings.LastIndex(arn, ":")+1:] }

func sortAttrs(a []attribute.KeyValue) []attribute.KeyValue {
	sort.Slice(a, func(i, j int) bool { return a[i].Key < a[j].Key })
	return a
}

// --- reflection helpers: all are nil-safe and return ok=false when the field is absent, nil or empty ---

func isNonNilStructPtr(v any) bool {
	if v == nil {
		return false
	}
	rv := reflect.ValueOf(v)
	return rv.Kind() == reflect.Pointer && !rv.IsNil() && rv.Elem().Kind() == reflect.Struct
}

// fieldValue returns the named field of the struct v points to, dereferencing a pointer field.
func fieldValue(v any, name string) (reflect.Value, bool) {
	f := reflect.ValueOf(v).Elem().FieldByName(name)
	if !f.IsValid() {
		return reflect.Value{}, false
	}
	if f.Kind() == reflect.Pointer {
		if f.IsNil() {
			return reflect.Value{}, false
		}
		f = f.Elem()
	}
	return f, true
}

// stringField returns a non-empty string field (*string or a named string type such as types.Select).
func stringField(v any, name string) (string, bool) {
	f, ok := fieldValue(v, name)
	if !ok || f.Kind() != reflect.String || f.String() == "" {
		return "", false
	}
	return f.String(), true
}

func boolField(v any, name string) (bool, bool) {
	f, ok := fieldValue(v, name)
	if !ok || f.Kind() != reflect.Bool {
		return false, false
	}
	return f.Bool(), true
}

func intField(v any, name string) (int64, bool) {
	f, ok := fieldValue(v, name)
	if !ok {
		return 0, false
	}
	switch f.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return f.Int(), true
	}
	return 0, false
}

func lenField(v any, name string) (int, bool) {
	f, ok := fieldValue(v, name)
	if !ok || f.Kind() != reflect.Slice {
		return 0, false
	}
	return f.Len(), true
}

// mapKeysField returns the sorted string keys of a map field.
func mapKeysField(v any, name string) []string {
	f, ok := fieldValue(v, name)
	if !ok || f.Kind() != reflect.Map || f.Type().Key().Kind() != reflect.String {
		return nil
	}
	keys := make([]string, 0, f.Len())
	for _, k := range f.MapKeys() {
		keys = append(keys, k.String())
	}
	sort.Strings(keys)
	return keys
}
