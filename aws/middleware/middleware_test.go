package middleware_test

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/stretchr/testify/assert"

	awsmiddleware "github.com/ellogroup/ello-golang-otel/aws/middleware"
)

func TestAppendToConfig_AddsMiddlewares(t *testing.T) {
	cfg := aws.Config{}
	before := len(cfg.APIOptions)

	awsmiddleware.AppendToConfig(&cfg)

	assert.Greater(t, len(cfg.APIOptions), before)
}
